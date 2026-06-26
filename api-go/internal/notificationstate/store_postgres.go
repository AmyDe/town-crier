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
	UnreadCount(ctx context.Context, userID string, lastReadAt time.Time) (int, error)
	DeleteByUserID(ctx context.Context, userID string) error
}

// Compile-time parity: *PostgresStore must satisfy the full Store surface.
var _ Store = (*PostgresStore)(nil)

// PostgresStore reads and writes per-user notification watermarks in the
// `notification_state` table and cross-reads the `notifications` table for
// the unread count — matching the Cosmos model where CosmosStore spans two
// containers (NotificationState + Notifications). Both tables live in the
// same pool, so the cross-read is a plain SQL join on the same connection.
//
// Partition strategy (vs Cosmos):
//   - The Cosmos model uses id == partition key == userId (one document per
//     user). The Postgres equivalent is `user_id TEXT PRIMARY KEY`.
//   - UnreadCount issues a SELECT count(*) against the notifications table,
//     replacing the Cosmos cross-container CountItems call.
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

// pgUnreadCountQuery counts notifications created strictly after the watermark
// for the given user, cross-reading the notifications table — the Postgres
// equivalent of the Cosmos cross-container CountItems call in CosmosStore.
const pgUnreadCountQuery = "SELECT count(*) FROM notifications WHERE user_id = $1 AND created_at > $2"

// UnreadCount counts the user's notifications created strictly after
// lastReadAt, matching the Cosmos CountItems semantics (the boundary instant
// itself counts as read).
func (s *PostgresStore) UnreadCount(ctx context.Context, userID string, lastReadAt time.Time) (int, error) {
	rows, err := s.db.Query(ctx, pgUnreadCountQuery, userID, lastReadAt)
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
