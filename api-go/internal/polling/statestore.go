package polling

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// pollStateCrossPartitionQuery selects every poll-state document for the
// least-recently-polled scan. The PollState container is partitioned by /id, so
// the LRU ordering needs a cross-partition read.
const pollStateCrossPartitionQuery = "SELECT * FROM c WHERE STARTSWITH(c.id, 'poll-state-')"

// stateItems is the consumer-side slice of the PollState container the store
// uses: a point read/upsert keyed on the per-authority document id, and a
// cross-partition scan for the LRU ordering. platform.CosmosContainer satisfies
// it structurally.
type stateItems interface {
	ReadItem(ctx context.Context, partitionKey, id string) ([]byte, error)
	UpsertItem(ctx context.Context, partitionKey string, item []byte) error
	QueryItemsCrossPartition(ctx context.Context, query string, params map[string]any) ([][]byte, error)
}

// pollStateDocument is the Cosmos persistence shape for a PollState. JSON tags
// use camelCase to match the stored document shape. Times use ISO 8601 with a
// numeric UTC offset (see dotNetRoundTrip); the cursor date uses yyyy-MM-dd.
// The container is partitioned by /id.
type pollStateDocument struct {
	ID            string  `json:"id"`
	LastPollTime  string  `json:"lastPollTime"`
	AuthorityID   int     `json:"authorityId"`
	HighWaterMark *string `json:"highWaterMark"`

	CursorDifferentStart *string `json:"cursorDifferentStart"`
	CursorNextPage       *int    `json:"cursorNextPage"`
	CursorKnownTotal     *int    `json:"cursorKnownTotal"`
}

// dotNetRoundTrip is the time layout for PollState timestamps: ISO 8601 with a
// 7-digit fractional second and a numeric UTC offset (e.g. "+00:00").
const dotNetRoundTrip = "2006-01-02T15:04:05.0000000-07:00"

// PollStateStore reads and writes per-authority poll state in the PollState
// container.
type PollStateStore struct {
	items stateItems
}

// NewPollStateStore returns a store backed by the given Cosmos item accessor.
func NewPollStateStore(items stateItems) *PollStateStore {
	return &PollStateStore{items: items}
}

// Get point-reads the poll state for authorityID. The boolean reports presence:
// a missing document is a normal "never polled" state, not an error.
func (s *PollStateStore) Get(ctx context.Context, authorityID int) (PollState, bool, error) {
	id := documentID(authorityID)
	raw, err := s.items.ReadItem(ctx, id, id)
	if err != nil {
		if platform.IsCosmosNotFound(err) {
			return PollState{}, false, nil
		}
		return PollState{}, false, fmt.Errorf("read poll state %d: %w", authorityID, err)
	}
	var doc pollStateDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return PollState{}, false, fmt.Errorf("decode poll state %d: %w", authorityID, err)
	}
	state, err := doc.toState()
	if err != nil {
		return PollState{}, false, fmt.Errorf("hydrate poll state %d: %w", authorityID, err)
	}
	return state, true, nil
}

// Save upserts the poll state for authorityID. A nil cursor clears any active
// cursor. The three poll-state fields are written together as a set.
func (s *PollStateStore) Save(ctx context.Context, authorityID int, lastPollTime, highWaterMark time.Time, cursor *PollCursor) error {
	id := documentID(authorityID)
	hwm := highWaterMark.UTC().Format(dotNetRoundTrip)
	doc := pollStateDocument{
		ID:            id,
		LastPollTime:  lastPollTime.UTC().Format(dotNetRoundTrip),
		AuthorityID:   authorityID,
		HighWaterMark: &hwm,
	}
	if cursor != nil {
		ds := cursor.DifferentStart.UTC().Format("2006-01-02")
		doc.CursorDifferentStart = &ds
		np := cursor.NextPage
		doc.CursorNextPage = &np
		doc.CursorKnownTotal = cursor.KnownTotal
	}
	body, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("encode poll state %d: %w", authorityID, err)
	}
	if err := s.items.UpsertItem(ctx, id, body); err != nil {
		return fmt.Errorf("upsert poll state %d: %w", authorityID, err)
	}
	return nil
}

// GetLeastRecentlyPolled returns the candidate authority ids ordered
// never-polled-first then ascending LastPollTime, plus the never-polled count.
// One cross-partition scan over all poll-state docs, then an in-memory sort of
// the candidate set.
func (s *PollStateStore) GetLeastRecentlyPolled(ctx context.Context, candidateAuthorityIDs []int) (LeastRecentlyPolledResult, error) {
	if len(candidateAuthorityIDs) == 0 {
		return LeastRecentlyPolledResult{AuthorityIDs: []int{}, NeverPolledCount: 0}, nil
	}

	raws, err := s.items.QueryItemsCrossPartition(ctx, pollStateCrossPartitionQuery, nil)
	if err != nil {
		return LeastRecentlyPolledResult{}, fmt.Errorf("query poll state: %w", err)
	}

	lastPollByAuthority := make(map[int]time.Time, len(raws))
	for _, raw := range raws {
		var doc pollStateDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return LeastRecentlyPolledResult{}, fmt.Errorf("decode poll state row: %w", err)
		}
		lpt, err := time.Parse(time.RFC3339Nano, doc.LastPollTime)
		if err != nil {
			return LeastRecentlyPolledResult{}, fmt.Errorf("parse lastPollTime %q: %w", doc.LastPollTime, err)
		}
		lastPollByAuthority[doc.AuthorityID] = lpt
	}

	sorted := make([]int, len(candidateAuthorityIDs))
	copy(sorted, candidateAuthorityIDs)
	// Stable sort: never-polled (rank 0) before polled (rank 1), then by oldest
	// LastPollTime ascending.
	sort.SliceStable(sorted, func(i, j int) bool {
		ai, aj := sorted[i], sorted[j]
		ti, polledI := lastPollByAuthority[ai]
		tj, polledJ := lastPollByAuthority[aj]
		if polledI != polledJ {
			// never-polled (false) sorts first
			return !polledI
		}
		if !polledI {
			return false // both never-polled: stable, keep input order
		}
		return ti.Before(tj)
	})

	neverPolled := 0
	for _, id := range candidateAuthorityIDs {
		if _, ok := lastPollByAuthority[id]; !ok {
			neverPolled++
		}
	}

	return LeastRecentlyPolledResult{AuthorityIDs: sorted, NeverPolledCount: neverPolled}, nil
}

// toState hydrates a stored document into a PollState. A legacy document missing
// highWaterMark falls back to lastPollTime.
func (d pollStateDocument) toState() (PollState, error) {
	lpt, err := time.Parse(time.RFC3339Nano, d.LastPollTime)
	if err != nil {
		return PollState{}, fmt.Errorf("parse lastPollTime %q: %w", d.LastPollTime, err)
	}
	hwm := lpt
	if d.HighWaterMark != nil {
		hwm, err = time.Parse(time.RFC3339Nano, *d.HighWaterMark)
		if err != nil {
			return PollState{}, fmt.Errorf("parse highWaterMark %q: %w", *d.HighWaterMark, err)
		}
	}
	cursor, err := d.readCursor()
	if err != nil {
		return PollState{}, err
	}
	return PollState{LastPollTime: lpt, HighWaterMark: hwm, Cursor: cursor}, nil
}

// readCursor reconstitutes the cursor; all three fields move as a set, so an
// absent date or page means there is no active cursor.
func (d pollStateDocument) readCursor() (*PollCursor, error) {
	if d.CursorDifferentStart == nil || d.CursorNextPage == nil {
		return nil, nil //nolint:nilnil // absence of a cursor is a valid nil, not an error
	}
	ds, err := time.Parse("2006-01-02", *d.CursorDifferentStart)
	if err != nil {
		return nil, fmt.Errorf("parse cursorDifferentStart %q: %w", *d.CursorDifferentStart, err)
	}
	return &PollCursor{DifferentStart: ds, NextPage: *d.CursorNextPage, KnownTotal: d.CursorKnownTotal}, nil
}

// documentID formats the per-authority PollState document id (== partition key).
func documentID(authorityID int) string {
	return "poll-state-" + strconv.Itoa(authorityID)
}
