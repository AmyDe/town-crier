package subscriptions

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// querier is the consumer-side slice of *pgxpool.Pool the Postgres store uses:
// parameterised exec/query/query-row. Defining it here (not importing pgxpool)
// keeps the store decoupled from the concrete pool and lets a pgx.Tx stand in.
// Both *pgxpool.Pool and pgx.Tx satisfy it structurally.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Store is the full method set the apple-notification idempotency store exposes
// to its consumers. It serves two purposes: a compile-time parity check (both
// *CosmosNotificationStore and *PostgresNotificationStore must satisfy it, so a
// Postgres port can never silently diverge from the Cosmos surface) and the
// exported consumer-side interface cmd/api's wiring will accept so the store can
// be flag-selected between backends (a later slice wires the flip).
type Store interface {
	IsProcessed(ctx context.Context, notificationUUID string) (bool, error)
	MarkProcessed(ctx context.Context, notificationUUID string) error
}

// Compile-time parity: both the Cosmos and Postgres stores satisfy the exported
// Store interface and the handler's package-local idempotencyStore interface.
var (
	_ Store            = (*CosmosNotificationStore)(nil)
	_ Store            = (*PostgresNotificationStore)(nil)
	_ idempotencyStore = (*PostgresNotificationStore)(nil)
)

// PostgresNotificationStore records and detects processed App Store Server
// Notifications in the apple_notifications table (Cosmos -> Postgres migration;
// memo 0010, epic #645). It is a parallel implementation: Cosmos remains wired
// until the flag flip in a later slice.
//
// The natural key is notification_uuid (the UUID Apple supplies on every
// delivery), so IsProcessed is a single EXISTS point read and MarkProcessed is
// an INSERT ... ON CONFLICT (notification_uuid) DO UPDATE — last-writer-wins,
// so a re-delivery that races past IsProcessed is silently absorbed.
type PostgresNotificationStore struct {
	db  querier
	now func() time.Time
}

// NewPostgresNotificationStore returns a store over the given pgx pool (or any
// querier). now supplies the processed_at timestamp for MarkProcessed (injected
// for deterministic tests).
func NewPostgresNotificationStore(db querier, now func() time.Time) *PostgresNotificationStore {
	return &PostgresNotificationStore{db: db, now: now}
}

const isProcessedQuery = "SELECT EXISTS(SELECT 1 FROM apple_notifications WHERE notification_uuid = $1)"

// IsProcessed reports whether the notification UUID has already been handled.
// It maps a SELECT EXISTS result directly to bool; the database guarantees this
// query always returns exactly one row, so ErrNoRows cannot occur.
func (s *PostgresNotificationStore) IsProcessed(ctx context.Context, notificationUUID string) (bool, error) {
	var processed bool
	if err := s.db.QueryRow(ctx, isProcessedQuery, notificationUUID).Scan(&processed); err != nil {
		return false, fmt.Errorf("check processed notification %q: %w", notificationUUID, err)
	}
	return processed, nil
}

// markProcessedQuery inserts or silently refreshes an apple_notifications row.
// ON CONFLICT DO UPDATE makes this last-writer-wins idempotent: a duplicate
// delivery that races past IsProcessed is absorbed without a constraint error,
// matching the Cosmos UpsertItem last-writer-wins semantics exactly.
const markProcessedQuery = `
INSERT INTO apple_notifications (notification_uuid, processed_at)
VALUES ($1, $2)
ON CONFLICT (notification_uuid) DO UPDATE SET processed_at = EXCLUDED.processed_at`

// MarkProcessed records the notification as handled. It uses now() as the
// processed_at timestamp. For idempotency under concurrent delivery see the
// SQL constant above.
func (s *PostgresNotificationStore) MarkProcessed(ctx context.Context, notificationUUID string) error {
	_, err := s.db.Exec(ctx, markProcessedQuery, notificationUUID, s.now())
	if err != nil {
		return fmt.Errorf("mark processed notification %q: %w", notificationUUID, err)
	}
	return nil
}

// upsertProcessedQuery is identical to markProcessedQuery but named separately
// for clarity: the backfill uses it with the original processedAt from Cosmos
// rather than s.now(), preserving historical timestamps across the migration.
const upsertProcessedQuery = `
INSERT INTO apple_notifications (notification_uuid, processed_at)
VALUES ($1, $2)
ON CONFLICT (notification_uuid) DO UPDATE SET processed_at = EXCLUDED.processed_at`

// UpsertProcessed inserts a processed notification with the given processedAt
// timestamp. This is used exclusively by the Cosmos -> Postgres backfill
// (cmd/pgbackfill-applenotifs) to preserve the original processing timestamp;
// it is not part of the Store interface (the runtime path uses MarkProcessed
// with the injected now clock).
func (s *PostgresNotificationStore) UpsertProcessed(ctx context.Context, notificationUUID string, processedAt time.Time) error {
	_, err := s.db.Exec(ctx, upsertProcessedQuery, notificationUUID, processedAt)
	if err != nil {
		return fmt.Errorf("upsert processed notification %q: %w", notificationUUID, err)
	}
	return nil
}
