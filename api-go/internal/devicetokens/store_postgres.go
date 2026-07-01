package devicetokens

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// querier is the consumer-side slice of *pgxpool.Pool the store uses:
// parameterised exec/query/query-row. Both *pgxpool.Pool and pgx.Tx satisfy it
// structurally, so the store is testable without a real connection.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Store is the full device-registration method set *PostgresStore satisfies and
// the exported consumer-side interface the handlers and wiring depend on.
//
// PurgeOlderThan is deliberately NOT in Store: it backs the pg-purge retention
// job and exists only on *PostgresStore.
type Store interface {
	GetByToken(ctx context.Context, userID, token string) (*DeviceRegistration, error)
	Save(ctx context.Context, reg DeviceRegistration) error
	Delete(ctx context.Context, userID, token string) error
	ListByUser(ctx context.Context, userID string) ([]DeviceRegistration, error)
	DeleteAllByUserID(ctx context.Context, userID string) error
}

// Compile-time check: the store satisfies the consumer-side Store interface.
var _ Store = (*PostgresStore)(nil)

// PostgresStore reads and writes device registrations in the Postgres
// `device_registrations` table (Cosmos → Postgres migration; memo 0010, epic #645).
//
// Partition strategy: the Cosmos container is partitioned by /userId with document
// id == token; the natural PK here is (user_id, token), matching exactly.
// PurgeOlderThan replaces the Cosmos 180-day TTL: registered_at is the ageing field.
type PostgresStore struct {
	db querier
}

// NewPostgresStore returns a store backed by the given pgx pool or querier.
func NewPostgresStore(db querier) *PostgresStore {
	return &PostgresStore{db: db}
}

const pgGetByTokenQuery = "SELECT user_id, token, platform, registered_at " +
	"FROM device_registrations WHERE user_id = $1 AND token = $2"

// GetByToken point-reads the registration for (userID, token). A missing row
// returns (nil, nil) — the "not registered yet" signal the PUT handler branches on;
// any other failure is wrapped.
func (s *PostgresStore) GetByToken(ctx context.Context, userID, token string) (*DeviceRegistration, error) {
	var (
		uid          string
		tok          string
		platformStr  string
		registeredAt time.Time
	)
	err := s.db.QueryRow(ctx, pgGetByTokenQuery, userID, token).Scan(&uid, &tok, &platformStr, &registeredAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil // absent registration is a valid "not found" signal, not an error
	}
	if err != nil {
		return nil, fmt.Errorf("read device token %q: %w", token, err)
	}
	platform, err := ParsePlatform(platformStr)
	if err != nil {
		return nil, fmt.Errorf("parse platform for device token %q: %w", token, err)
	}
	return &DeviceRegistration{
		UserID:       uid,
		Token:        tok,
		Platform:     platform,
		RegisteredAt: registeredAt,
	}, nil
}

const pgSaveDeviceQuery = `
INSERT INTO device_registrations (user_id, token, platform, registered_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, token) DO UPDATE SET
    platform      = EXCLUDED.platform,
    registered_at = EXCLUDED.registered_at`

// Save upserts the device registration keyed on (user_id, token). A re-PUT
// resets registered_at (the ageing field for PurgeOlderThan) to the client's
// current instant, matching the Cosmos TTL-reset semantics.
func (s *PostgresStore) Save(ctx context.Context, reg DeviceRegistration) error {
	if _, err := s.db.Exec(ctx, pgSaveDeviceQuery,
		reg.UserID, reg.Token, reg.Platform.String(), reg.RegisteredAt); err != nil {
		return fmt.Errorf("upsert device token %q: %w", reg.Token, err)
	}
	return nil
}

const pgDeleteDeviceQuery = "DELETE FROM device_registrations WHERE user_id = $1 AND token = $2"

// Delete removes the registration for (userID, token). A missing row is not an
// error: the DELETE endpoint is idempotent (the token may already be gone).
func (s *PostgresStore) Delete(ctx context.Context, userID, token string) error {
	if _, err := s.db.Exec(ctx, pgDeleteDeviceQuery, userID, token); err != nil {
		return fmt.Errorf("delete device token %q: %w", token, err)
	}
	return nil
}

const pgListByUserQuery = "SELECT user_id, token, platform, registered_at " +
	"FROM device_registrations WHERE user_id = $1 ORDER BY registered_at"

// ListByUser returns every registration in the user's partition, ordered by
// registered_at. Used by the GDPR data-export path.
func (s *PostgresStore) ListByUser(ctx context.Context, userID string) ([]DeviceRegistration, error) {
	rows, err := s.db.Query(ctx, pgListByUserQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("query device tokens for %q: %w", userID, err)
	}
	defer rows.Close()

	var regs []DeviceRegistration
	for rows.Next() {
		var (
			uid          string
			tok          string
			platformStr  string
			registeredAt time.Time
		)
		if err := rows.Scan(&uid, &tok, &platformStr, &registeredAt); err != nil {
			return nil, fmt.Errorf("scan device token for %q: %w", userID, err)
		}
		platform, err := ParsePlatform(platformStr)
		if err != nil {
			return nil, fmt.Errorf("parse platform for device token %q: %w", tok, err)
		}
		regs = append(regs, DeviceRegistration{
			UserID:       uid,
			Token:        tok,
			Platform:     platform,
			RegisteredAt: registeredAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("device token rows for %q: %w", userID, err)
	}
	return regs, nil
}

// pgDeviceCountsByUsersQuery tallies each user's live device-registration count
// in one grouped pass. The (user_id, token) primary key makes each row a
// distinct live token, so count(*) per user is already deduped on (user_id,
// token). Users with no rows produce no row; the caller defaults them to 0.
const pgDeviceCountsByUsersQuery = "SELECT user_id, count(*) FROM device_registrations " +
	"WHERE user_id = ANY($1) GROUP BY user_id"

// CountsByUsers returns each user's live device-token count in a single grouped
// query, mirroring notifications.PostgresStore.CountsByUsers. The count reflects
// live (non-TTL-purged) tokens, deduped on (user_id, token) by the primary key.
// Users absent from the result are absent from the map; the caller treats a
// missing key as 0 via the map zero value. An empty user set returns an empty
// map without issuing a query.
func (s *PostgresStore) CountsByUsers(ctx context.Context, userIDs []string) (map[string]int, error) {
	counts := make(map[string]int, len(userIDs))
	if len(userIDs) == 0 {
		return counts, nil
	}
	rows, err := s.db.Query(ctx, pgDeviceCountsByUsersQuery, userIDs)
	if err != nil {
		return nil, fmt.Errorf("query device token counts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			userID string
			count  int
		)
		if err := rows.Scan(&userID, &count); err != nil {
			return nil, fmt.Errorf("scan device token count: %w", err)
		}
		counts[userID] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("device token count rows: %w", err)
	}
	return counts, nil
}

const pgDeviceCountQuery = "SELECT count(*) FROM device_registrations"

// Count returns the global number of live device registrations across all users
// — the admin stats "reach" total.
func (s *PostgresStore) Count(ctx context.Context) (int, error) {
	var total int
	if err := s.db.QueryRow(ctx, pgDeviceCountQuery).Scan(&total); err != nil {
		return 0, fmt.Errorf("count device registrations: %w", err)
	}
	return total, nil
}

const pgDeleteAllDevicesQuery = "DELETE FROM device_registrations WHERE user_id = $1"

// DeleteAllByUserID removes every device registration for the user. Used by the
// GDPR erasure cascade (dormant cleanup and DELETE /v1/me).
func (s *PostgresStore) DeleteAllByUserID(ctx context.Context, userID string) error {
	if _, err := s.db.Exec(ctx, pgDeleteAllDevicesQuery, userID); err != nil {
		return fmt.Errorf("delete all device tokens for %q: %w", userID, err)
	}
	return nil
}

const pgPurgeDevicesQuery = "DELETE FROM device_registrations WHERE registered_at < $1"

// PurgeOlderThan deletes every registration whose registered_at is before cutoff
// and returns the number of rows deleted. It replaces the Cosmos 180-day TTL: a
// caller schedules this with time.Now().Add(-180 * 24 * time.Hour) to enforce the
// UK GDPR Art. 5(1)(e) storage limitation for device identifiers.
func (s *PostgresStore) PurgeOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := s.db.Exec(ctx, pgPurgeDevicesQuery, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purge device tokens older than %v: %w", cutoff, err)
	}
	return tag.RowsAffected(), nil
}
