package notificationstate

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// querier omits QueryRow so unit-test fakes need no concrete pgx.Row; all reads
// use Query + CollectRows.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Store is the method set notificationstate consumers depend on.
type Store interface {
	Get(ctx context.Context, userID string) (*State, error)
	Save(ctx context.Context, st State) error
	UnreadCount(ctx context.Context, userID string) (int, error)
	MarkAllRead(ctx context.Context, userID string, now time.Time) (int64, error)
	MarkApplicationsRead(ctx context.Context, userID string, refs []string, authorityIDs []int, now time.Time) (int64, error)
	MarkReadUpTo(ctx context.Context, userID string, asOf, now time.Time) (int64, error)
	DeleteByUserID(ctx context.Context, userID string) error
}

var _ Store = (*PostgresStore)(nil)

// PostgresStore owns the notification_state change-token row and the read_at
// mutations over notifications. Each mutation and its version bump run
// atomically in one data-modifying CTE statement.
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
// the first-touch signal the handlers branch on.
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

// Save upserts the user's state row.
func (s *PostgresStore) Save(ctx context.Context, st State) error {
	if _, err := s.db.Exec(ctx, pgSaveStateQuery, st.UserID, st.LastReadAt, st.Version); err != nil {
		return fmt.Errorf("upsert notification state %q: %w", st.UserID, err)
	}
	return nil
}

// pgUnreadCountQuery is served by the partial index idx_notifications_unread.
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

// pgMarkApplicationsReadQuery matches application_name (the PlanIt case
// reference every client sends), not application_uid; authority_id
// disambiguates references that collide across councils. Empty arrays match
// nothing, never "all".
const pgMarkApplicationsReadQuery = `
WITH cleared AS (
    UPDATE notifications n
    SET read_at = $2
    FROM unnest($3::text[], $4::int[]) AS t(ref, aid)
    WHERE n.user_id = $1
      AND n.application_name = t.ref
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

// MarkApplicationsRead clears the caller's unread notifications for the
// (refs[i], authorityIDs[i]) pairs and returns the number cleared. The version
// token bumps only when a row was cleared, so a repeat call is a no-op.
func (s *PostgresStore) MarkApplicationsRead(ctx context.Context, userID string, refs []string, authorityIDs []int, now time.Time) (int64, error) {
	rows, err := s.db.Query(ctx, pgMarkApplicationsReadQuery, userID, now, refs, authorityIDs)
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

// pgMarkReadUpToQuery backs the advance compat shim. $1 userID, $2 asOf, $3 now.
const pgMarkReadUpToQuery = `
WITH cleared AS (
    UPDATE notifications
    SET read_at = $3
    WHERE user_id = $1 AND created_at <= $2 AND read_at IS NULL
    RETURNING 1
), bumped AS (
    INSERT INTO notification_state (user_id, last_read_at, version)
    SELECT $1, $3, 1 WHERE EXISTS (SELECT 1 FROM cleared)
    ON CONFLICT (user_id) DO UPDATE SET version = notification_state.version + 1
    RETURNING 1
)
SELECT count(*) FROM cleared`

// MarkReadUpTo clears every unread notification for the user created at or
// before asOf and returns the number cleared. The version token bumps only when
// a row was cleared. Only the advance compat shim calls it.
func (s *PostgresStore) MarkReadUpTo(ctx context.Context, userID string, asOf, now time.Time) (int64, error) {
	rows, err := s.db.Query(ctx, pgMarkReadUpToQuery, userID, asOf, now)
	if err != nil {
		return 0, fmt.Errorf("mark read up to for %q: %w", userID, err)
	}
	counts, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return 0, fmt.Errorf("scan mark read up to count for %q: %w", userID, err)
	}
	if len(counts) == 0 {
		return 0, nil
	}
	return counts[0], nil
}

const pgDeleteStateQuery = "DELETE FROM notification_state WHERE user_id = $1"

// DeleteByUserID removes the user's watermark — the GDPR Art. 17 erasure
// cascade (bridged to erasure.ChildDeleter by erasure.NotificationStateChild).
// A missing row is not an error.
func (s *PostgresStore) DeleteByUserID(ctx context.Context, userID string) error {
	if _, err := s.db.Exec(ctx, pgDeleteStateQuery, userID); err != nil {
		return fmt.Errorf("delete notification state %q: %w", userID, err)
	}
	return nil
}
