package polling

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// querier is the slice of *pgxpool.Pool the polling stores use; a pgx.Tx also
// satisfies it.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// LeaseAccess is the lease-store method set the worker wiring depends on.
type LeaseAccess interface {
	TryAcquire(ctx context.Context, ttl time.Duration) (LeaseAcquireResult, error)
	Release(ctx context.Context, handle LeaseHandle) LeaseReleaseOutcome
	Confirm(ctx context.Context, handle LeaseHandle, extend time.Duration) bool
}

var _ LeaseAccess = (*PostgresLeaseStore)(nil)

// PostgresLeaseStore is the Postgres-backed polling lease: one row in leases
// keyed by id='polling', taken and released by conditional SQL. LeaseHandle.ETag
// holds this process's random holder id, which gates Release and Confirm so one
// worker cannot release or extend another's lease.
type PostgresLeaseStore struct {
	db       querier
	now      func() time.Time
	holderID string
}

// NewPostgresLeaseStore wires the store.
// Each instance generates a fresh random holder id.
func NewPostgresLeaseStore(db querier, now func() time.Time) *PostgresLeaseStore {
	return &PostgresLeaseStore{
		db:       db,
		now:      now,
		holderID: newHolderID(),
	}
}

// tryAcquireLeaseQuery takes over an existing row only when its expires_at is
// at or before $5 (now); a live peer lease makes the WHERE false, so RETURNING
// yields no row.
const tryAcquireLeaseQuery = `
INSERT INTO leases (id, holder_id, acquired_at, expires_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (id) DO UPDATE SET
    holder_id   = EXCLUDED.holder_id,
    acquired_at = EXCLUDED.acquired_at,
    expires_at  = EXCLUDED.expires_at
WHERE leases.expires_at <= $5
RETURNING holder_id`

const releaseLeaseQuery = `DELETE FROM leases WHERE id = $1 AND holder_id = $2`

// leaseExistsQuery lets Release tell "already gone" from "held by a peer".
const leaseExistsQuery = `SELECT COUNT(*) FROM leases WHERE id = $1`

// TryAcquire attempts to acquire the polling lease for ttl. A peer holding it is
// reported as Held and a database failure on TransientErr; the returned error is
// always nil.
func (s *PostgresLeaseStore) TryAcquire(ctx context.Context, ttl time.Duration) (LeaseAcquireResult, error) {
	now := s.now().UTC()
	expiresAt := now.Add(ttl)

	var holderID string
	err := s.db.QueryRow(ctx, tryAcquireLeaseQuery,
		leaseDocumentID,
		s.holderID,
		now,
		expiresAt,
		now,
	).Scan(&holderID)

	if errors.Is(err, pgx.ErrNoRows) {
		return LeaseAcquireResult{Held: true}, nil
	}
	if err != nil {
		return LeaseAcquireResult{TransientErr: fmt.Errorf("acquire polling lease: %w", err)}, nil
	}
	return LeaseAcquireResult{Acquired: true, Handle: LeaseHandle{ETag: holderID}}, nil
}

// confirmLeaseQuery checks ownership and liveness and extends expiry in one
// statement, so there is no gap between the check and a subsequent publish.
const confirmLeaseQuery = `
UPDATE leases SET expires_at = $4
WHERE id = $1 AND holder_id = $2 AND expires_at > $3
RETURNING holder_id`

// Confirm reports whether handle's holder still owns a live polling lease, and
// if so atomically extends its expiry by extend from now. It never returns a
// hard error — a lost lease (expired, held by a peer, or a database failure) all
// report false, so a caller's next-trigger publish gate degrades to "skip
// publish".
func (s *PostgresLeaseStore) Confirm(ctx context.Context, handle LeaseHandle, extend time.Duration) bool {
	now := s.now().UTC()
	newExpiresAt := now.Add(extend)

	var holderID string
	err := s.db.QueryRow(ctx, confirmLeaseQuery,
		leaseDocumentID,
		handle.ETag,
		now,
		newExpiresAt,
	).Scan(&holderID)

	return err == nil
}

// Release performs a conditional delete using the handle's holder id. It never
// returns a hard error — failures surface as an outcome, and the lease TTL is
// the backstop for any non-Released outcome.
func (s *PostgresLeaseStore) Release(ctx context.Context, handle LeaseHandle) LeaseReleaseOutcome {
	tag, err := s.db.Exec(ctx, releaseLeaseQuery, leaseDocumentID, handle.ETag)
	if err != nil {
		return LeaseTransientError
	}
	if tag.RowsAffected() == 1 {
		return LeaseReleased
	}

	var count int
	if err := s.db.QueryRow(ctx, leaseExistsQuery, leaseDocumentID).Scan(&count); err != nil {
		return LeaseTransientError
	}
	if count == 0 {
		return LeaseAlreadyGone
	}
	return LeasePreconditionFailed
}
