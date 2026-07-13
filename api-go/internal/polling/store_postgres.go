package polling

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// querier is the consumer-side slice of *pgxpool.Pool the polling stores use:
// parameterised exec/query/query-row. Defining it here (not importing pgxpool)
// keeps the stores decoupled from the concrete pool and lets a pgx.Tx stand in.
// Both *pgxpool.Pool and pgx.Tx satisfy it structurally.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// PollStateAccess is the full poll-state store method set its consumers rely on
// and the exported consumer-side interface the worker wiring depends on.
type PollStateAccess interface {
	Get(ctx context.Context, authorityID int) (PollState, bool, error)
	Save(ctx context.Context, authorityID int, lastPollTime, highWaterMark time.Time, cursor *PollCursor) error
	GetLeastRecentlyPolled(ctx context.Context, candidateAuthorityIDs []int) (LeastRecentlyPolledResult, error)
}

// Compile-time check: the store satisfies the consumer-side interface.
var _ PollStateAccess = (*PostgresPollStateStore)(nil)

// PostgresPollStateStore reads and writes per-authority poll state in the
// Postgres `poll_state` table (Cosmos -> Postgres migration; memo 0010, epic
// #645). It is a parallel implementation of *PollStateStore.
//
// The LRU scan (GetLeastRecentlyPolled) pushes sorting and never-polled detection
// into a single LEFT JOIN query, eliminating the in-memory sort the Cosmos
// implementation requires for its cross-partition result set.
type PostgresPollStateStore struct {
	db querier
}

// NewPostgresPollStateStore returns a store over the given pgx pool (or any
// querier — a pgx.Tx also satisfies the interface).
func NewPostgresPollStateStore(db querier) *PostgresPollStateStore {
	return &PostgresPollStateStore{db: db}
}

// legacyPageSize is the fixed page size the old page-granular cursor was
// recorded against (the pre-tc-nlvpz defaultPageSize). It is used ONLY to
// convert a legacy cursor_next_page value into an equivalent record index for
// rows written before this release: cursor_next_index = (cursor_next_page-1) *
// legacyPageSize, matching the migration's SQL backfill exactly.
const legacyPageSize = 100

// getPollStateQuery point-reads the poll state for one authority by its integer
// id. Cursor columns are nullable; absent cursor_different_start means no
// active cursor. Both the new cursor_next_index and the old cursor_next_page
// are read so Get can fall back to the legacy column for a row the additive
// migration backfilled but a concurrent old-code writer might still be
// touching during the deploy-overlap window (cursor_next_page is kept for one
// release after tc-nlvpz for exactly this reason).
const getPollStateQuery = `
SELECT last_poll_time, high_water_mark,
       cursor_different_start, cursor_next_index, cursor_next_page, cursor_known_total
FROM poll_state
WHERE authority_id = $1`

// Get point-reads the poll state for authorityID. The boolean reports presence:
// a missing row is the normal "never polled" state, not an error.
func (s *PostgresPollStateStore) Get(ctx context.Context, authorityID int) (PollState, bool, error) {
	var (
		lastPollTime     time.Time
		highWaterMark    time.Time
		cursorDiffStart  *time.Time
		cursorNextIndex  *int
		cursorNextPage   *int
		cursorKnownTotal *int
	)
	err := s.db.QueryRow(ctx, getPollStateQuery, authorityID).Scan(
		&lastPollTime, &highWaterMark,
		&cursorDiffStart, &cursorNextIndex, &cursorNextPage, &cursorKnownTotal,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return PollState{}, false, nil
	}
	if err != nil {
		return PollState{}, false, fmt.Errorf("read poll state %d: %w", authorityID, err)
	}

	state := PollState{
		LastPollTime:  lastPollTime.UTC(),
		HighWaterMark: highWaterMark.UTC(),
	}
	if cursorDiffStart != nil {
		nextIndex := 0
		switch {
		case cursorNextIndex != nil:
			nextIndex = *cursorNextIndex
		case cursorNextPage != nil:
			// Legacy row: only the old page-granular column is set (the
			// migration's backfill missed it, or it predates the migration).
			// Convert on read so callers never see the old shape.
			nextIndex = (*cursorNextPage - 1) * legacyPageSize
		}
		state.Cursor = &PollCursor{
			DifferentStart: cursorDiffStart.UTC(),
			NextIndex:      nextIndex,
			KnownTotal:     cursorKnownTotal,
		}
	}
	return state, true, nil
}

// savePollStateQuery upserts the poll state for one authority. Cursor columns
// are written together as a set; a nil cursor clears all cursor fields to
// NULL, which Get interprets as "no active cursor". cursor_next_page is always
// nulled on write — every save migrates the row forward to the index-only
// shape, matching the "writes always set cursor_next_index and null out
// cursor_next_page" plan (tc-nlvpz).
const savePollStateQuery = `
INSERT INTO poll_state (
    authority_id, last_poll_time, high_water_mark,
    cursor_different_start, cursor_next_index, cursor_next_page, cursor_known_total
) VALUES ($1, $2, $3, $4, $5, NULL, $6)
ON CONFLICT (authority_id) DO UPDATE SET
    last_poll_time         = EXCLUDED.last_poll_time,
    high_water_mark        = EXCLUDED.high_water_mark,
    cursor_different_start = EXCLUDED.cursor_different_start,
    cursor_next_index      = EXCLUDED.cursor_next_index,
    cursor_next_page       = NULL,
    cursor_known_total     = EXCLUDED.cursor_known_total`

// Save upserts the poll state for authorityID. A nil cursor clears any active
// cursor. The poll-state fields are written together as a set, and every save
// nulls the legacy cursor_next_page column (writes always use the new
// cursor_next_index shape).
func (s *PostgresPollStateStore) Save(ctx context.Context, authorityID int, lastPollTime, highWaterMark time.Time, cursor *PollCursor) error {
	var (
		cursorDiffStart  *time.Time
		cursorNextIndex  *int
		cursorKnownTotal *int
	)
	if cursor != nil {
		ds := cursor.DifferentStart.UTC()
		cursorDiffStart = &ds
		cursorNextIndex = &cursor.NextIndex
		cursorKnownTotal = cursor.KnownTotal
	}

	_, err := s.db.Exec(ctx, savePollStateQuery,
		authorityID,
		lastPollTime.UTC(),
		highWaterMark.UTC(),
		cursorDiffStart,
		cursorNextIndex,
		cursorKnownTotal,
	)
	if err != nil {
		return fmt.Errorf("save poll state %d: %w", authorityID, err)
	}
	return nil
}

// getLRPQuery returns candidate authority ids ordered never-polled-first (NULL
// last_poll_time sorts before any real timestamp with NULLS FIRST) then by
// ascending last_poll_time. The unnest($1::integer[]) CTE drives the candidate
// set so authorities with no row in poll_state still appear in the output (with
// a NULL last_poll_time). The ORDER BY pushes what was an in-memory sort in the
// Cosmos implementation into the database.
const getLRPQuery = `
SELECT c.authority_id, ps.last_poll_time
FROM unnest($1::integer[]) AS c(authority_id)
LEFT JOIN poll_state ps USING (authority_id)
ORDER BY ps.last_poll_time ASC NULLS FIRST`

// GetLeastRecentlyPolled returns the candidate authority ids ordered
// never-polled-first then ascending LastPollTime, plus the never-polled count.
func (s *PostgresPollStateStore) GetLeastRecentlyPolled(ctx context.Context, candidateAuthorityIDs []int) (LeastRecentlyPolledResult, error) {
	if len(candidateAuthorityIDs) == 0 {
		return LeastRecentlyPolledResult{AuthorityIDs: []int{}, NeverPolledCount: 0}, nil
	}

	rows, err := s.db.Query(ctx, getLRPQuery, candidateAuthorityIDs)
	if err != nil {
		return LeastRecentlyPolledResult{}, fmt.Errorf("query least recently polled: %w", err)
	}
	defer rows.Close()

	var (
		ids         []int
		neverPolled int
	)
	for rows.Next() {
		var (
			authorityID  int
			lastPollTime *time.Time // NULL for never-polled authorities
		)
		if err := rows.Scan(&authorityID, &lastPollTime); err != nil {
			return LeastRecentlyPolledResult{}, fmt.Errorf("scan least recently polled row: %w", err)
		}
		ids = append(ids, authorityID)
		if lastPollTime == nil {
			neverPolled++
		}
	}
	if err := rows.Err(); err != nil {
		return LeastRecentlyPolledResult{}, fmt.Errorf("iterate least recently polled: %w", err)
	}

	if ids == nil {
		ids = []int{}
	}
	return LeastRecentlyPolledResult{AuthorityIDs: ids, NeverPolledCount: neverPolled}, nil
}
