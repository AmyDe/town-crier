package notificationstate

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// querier is the consumer-side slice of *pgxpool.Pool the store uses.
// Only Exec and Query are needed — all reads use Query + CollectRows,
// keeping the interface fakeable for unit tests. Both *pgxpool.Pool and
// pgx.Tx satisfy it structurally.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Store is the full exported method set the notificationstate consumers rely
// on. It serves two purposes: compile-time parity so *PostgresStore can never
// silently diverge from the method set callers need, and the consumer-side
// interface the API wiring accepts once the Postgres backend is selected.
type Store interface {
	Get(ctx context.Context, userID string) (*State, error)
	Save(ctx context.Context, st State) error
	UnreadCount(ctx context.Context, userID string) (int, error)
	MarkAllRead(ctx context.Context, userID string, now time.Time) (int64, error)
	MarkApplicationsRead(ctx context.Context, userID string, uids []string, authorityIDs []int, now time.Time) (int64, error)
	DeleteByUserID(ctx context.Context, userID string) error
}

// Compile-time parity: *PostgresStore must satisfy the full Store surface.
var _ Store = (*PostgresStore)(nil)

// PostgresStore owns the per-user notification read state: the change-token row
// in `notification_state` and the read-state mutations over the `notifications`
// table (read_at). Both tables live in the same pool, so a mutation and its
// version bump run atomically in a single data-modifying CTE statement.
//
//   - Unread is read_at IS NULL (ADR 0035); UnreadCount is a SELECT count(*)
//     over the partial index idx_notifications_unread.
//   - MarkApplicationsRead clears the caller's unread rows for a set of
//     (application_uid, authority_id) pairs — the composite is load-bearing:
//     application_uid is the bare per-council PlanIt ref and is NOT unique
//     across authorities, so matching on uid alone would clear the wrong
//     council's rows. It bumps the version token only when it cleared a row.
//   - MarkAllRead clears every unread row for the user and always bumps the
//     version token (upserting the state row when absent).
type PostgresStore struct {
	db querier
}

// NewPostgresStore returns a store over the given pgx pool (or any querier).
func NewPostgresStore(db querier) *PostgresStore {
	return &PostgresStore{db: db}
}

// scanStateRow hydrates one row from notification_state.
// Column order: user_id, last_read_at, version — MUST match all SELECT queries.
func scanStateRow(row pgx.CollectableRow) (State, error) {
	var (
		userID     string
		lastReadAt time.Time
		version    int
	)
	if err := row.Scan(&userID, &lastReadAt, &version); err != nil {
		return State{}, err
	}
	return State{
		UserID:     userID,
		LastReadAt: lastReadAt,
		Version:    version,
	}, nil
}

const pgGetStateQuery = "SELECT user_id, last_read_at, version FROM notification_state WHERE user_id = $1"

// Get point-reads the user's watermark. A missing row returns (nil, nil) —
// the first-touch signal the handlers branch on, identical to the Cosmos
// 404-on-ReadItem behaviour.
func (s *PostgresStore) Get(ctx context.Context, userID string) (*State, error) {
	rows, err := s.db.Query(ctx, pgGetStateQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("read notification state %q: %w", userID, err)
	}
	items, err := pgx.CollectRows(rows, scanStateRow)
	if err != nil {
		return nil, fmt.Errorf("scan notification state %q: %w", userID, err)
	}
	if len(items) == 0 {
		return nil, nil //nolint:nilnil // absent watermark is the first-touch signal, not an error
	}
	st := items[0]
	return &st, nil
}

const pgSaveStateQuery = `
INSERT INTO notification_state (user_id, last_read_at, version)
VALUES ($1, $2, $3)
ON CONFLICT (user_id) DO UPDATE SET
    last_read_at = EXCLUDED.last_read_at,
    version      = EXCLUDED.version`

// Save upserts the watermark, matching the Cosmos UpsertItem-by-userId
// contract.
func (s *PostgresStore) Save(ctx context.Context, st State) error {
	if _, err := s.db.Exec(ctx, pgSaveStateQuery, st.UserID, st.LastReadAt, st.Version); err != nil {
		return fmt.Errorf("upsert notification state %q: %w", st.UserID, err)
	}
	return nil
}

// pgUnreadCountQuery counts the user's unread notifications (read_at IS NULL),
// served by the partial index idx_notifications_unread (ADR 0035).
const pgUnreadCountQuery = "SELECT count(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL"

// UnreadCount counts the user's unread notifications (read_at IS NULL).
func (s *PostgresStore) UnreadCount(ctx context.Context, userID string) (int, error) {
	rows, err := s.db.Query(ctx, pgUnreadCountQuery, userID)
	if err != nil {
		return 0, fmt.Errorf("count unread for %q: %w", userID, err)
	}
	counts, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return 0, fmt.Errorf("scan unread count for %q: %w", userID, err)
	}
	if len(counts) == 0 {
		return 0, nil
	}
	return int(counts[0]), nil
}

// pgMarkAllReadQuery clears every unread notification for the user (read_at set
// to now) and unconditionally bumps the version change token, upserting the
// state row when the user has none. Both tables mutate atomically in one
// data-modifying CTE statement; the top-level SELECT returns the cleared count.
const pgMarkAllReadQuery = `
WITH cleared AS (
    UPDATE notifications
    SET read_at = $2
    WHERE user_id = $1 AND read_at IS NULL
    RETURNING 1
), bumped AS (
    INSERT INTO notification_state (user_id, last_read_at, version)
    VALUES ($1, $2, 1)
    ON CONFLICT (user_id) DO UPDATE SET
        last_read_at = EXCLUDED.last_read_at,
        version      = notification_state.version + 1
    RETURNING 1
)
SELECT count(*) FROM cleared`

// MarkAllRead clears all of the user's unread notifications and bumps the
// version change token (upserting the state row if absent). It returns the
// number of notifications cleared. The version bump is unconditional so
// BadgeSync still observes a change even when nothing was unread.
func (s *PostgresStore) MarkAllRead(ctx context.Context, userID string, now time.Time) (int64, error) {
	rows, err := s.db.Query(ctx, pgMarkAllReadQuery, userID, now)
	if err != nil {
		return 0, fmt.Errorf("mark all read for %q: %w", userID, err)
	}
	counts, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return 0, fmt.Errorf("scan mark all read count for %q: %w", userID, err)
	}
	if len(counts) == 0 {
		return 0, nil
	}
	return counts[0], nil
}

// pgMarkApplicationsReadQuery clears the caller's unread notifications for a set
// of (application_uid, authority_id) pairs supplied as two parallel arrays, and
// bumps the version change token only when it actually cleared a row (upserting
// the state row when absent). Scoping by the composite pair is load-bearing:
// application_uid is the bare per-council PlanIt ref and collides across
// authorities, so authority_id disambiguates. Empty arrays match nothing (a
// 204 no-op), never "all". Both tables mutate atomically in one CTE statement;
// the top-level SELECT returns the cleared count.
const pgMarkApplicationsReadQuery = `
WITH cleared AS (
    UPDATE notifications n
    SET read_at = $2
    FROM unnest($3::text[], $4::int[]) AS t(uid, aid)
    WHERE n.user_id = $1
      AND n.application_uid = t.uid
      AND n.authority_id = t.aid
      AND n.read_at IS NULL
    RETURNING 1
), bumped AS (
    INSERT INTO notification_state (user_id, last_read_at, version)
    SELECT $1, $2, 1
    WHERE EXISTS (SELECT 1 FROM cleared)
    ON CONFLICT (user_id) DO UPDATE SET version = notification_state.version + 1
    RETURNING 1
)
SELECT count(*) FROM cleared`

// MarkApplicationsRead clears the caller's unread notifications for the given
// (uid, authorityID) pairs and bumps the version change token when it cleared at
// least one row (leaving the token untouched on a zero-row no-op, so mark-read
// stays idempotent). uids and authorityIDs are parallel: the i-th pair is
// (uids[i], authorityIDs[i]). It returns the number of notifications cleared.
func (s *PostgresStore) MarkApplicationsRead(ctx context.Context, userID string, uids []string, authorityIDs []int, now time.Time) (int64, error) {
	rows, err := s.db.Query(ctx, pgMarkApplicationsReadQuery, userID, now, uids, authorityIDs)
	if err != nil {
		return 0, fmt.Errorf("mark applications read for %q: %w", userID, err)
	}
	counts, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return 0, fmt.Errorf("scan mark applications read count for %q: %w", userID, err)
	}
	if len(counts) == 0 {
		return 0, nil
	}
	return counts[0], nil
}

const pgDeleteStateQuery = "DELETE FROM notification_state WHERE user_id = $1"

// DeleteByUserID removes the user's watermark — the GDPR Art. 17 erasure
// cascade (bridged to erasure.ChildDeleter by erasure.NotificationStateChild).
// A missing row (no watermark yet) is not an error, matching the Cosmos
// 404-tolerant DeleteItem behaviour.
func (s *PostgresStore) DeleteByUserID(ctx context.Context, userID string) error {
	if _, err := s.db.Exec(ctx, pgDeleteStateQuery, userID); err != nil {
		return fmt.Errorf("delete notification state %q: %w", userID, err)
	}
	return nil
}
