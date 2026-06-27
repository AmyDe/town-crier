package polling

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// LeaseAccess is the full lease-store method set its consumers rely on and the
// exported consumer-side interface the worker wiring depends on.
type LeaseAccess interface {
	TryAcquire(ctx context.Context, ttl time.Duration) (LeaseAcquireResult, error)
	Release(ctx context.Context, handle LeaseHandle) LeaseReleaseOutcome
}

// Compile-time check: the store satisfies the consumer-side interface.
var _ LeaseAccess = (*PostgresLeaseStore)(nil)

// PostgresLeaseStore is the Postgres-backed polling lease (Cosmos → Postgres
// migration; memo 0010, epic #645). It maintains a single row in the `leases`
// table keyed by id='polling' and uses an atomic conditional INSERT/UPDATE to
// implement the same CAS semantics the Cosmos etag-CAS store provides.
//
// TryAcquire:
//   - If no row exists → INSERT wins → Acquired.
//   - If a row exists and its expires_at is in the past → UPDATE wins → Acquired.
//   - If a row exists and its expires_at is in the future → the WHERE condition
//     on ON CONFLICT DO UPDATE is false → no rows affected → RETURNING returns
//     nothing → Held.
//
// Release:
//   - DELETE WHERE id=$1 AND holder_id=$2 (handle carries our holder id).
//   - rows_affected=1 → LeaseReleased.
//   - rows_affected=0, row still present → LeasePreconditionFailed (different holder).
//   - rows_affected=0, row absent → LeaseAlreadyGone.
//
// The holder id stored in LeaseHandle.ETag is our process-unique random id,
// playing the same role as the Cosmos document etag: it gates the conditional
// delete so one worker cannot release another's lease.
type PostgresLeaseStore struct {
	db       querier
	now      func() time.Time
	holderID string
}

// NewPostgresLeaseStore wires the Postgres lease store. now is injected so
// tests pin the clock; production passes time.Now. A fresh random holder id is
// generated per store instance (one per process), written as diagnostic
// metadata and used as the conditional-delete token on Release.
func NewPostgresLeaseStore(db querier, now func() time.Time) *PostgresLeaseStore {
	return &PostgresLeaseStore{
		db:       db,
		now:      now,
		holderID: newHolderID(),
	}
}

// tryAcquireLeaseQuery atomically acquires the polling lease. On INSERT (no
// existing row) the new row is always written. On conflict the DO UPDATE only
// fires when the existing row's expires_at is at or before $5 (now), meaning
// the previous holder's lease has expired. If the existing row is live (expires
// in the future) the WHERE condition is false: no rows are affected and RETURNING
// returns nothing, signalling that the lease is held by a peer.
//
// Parameters: $1=id, $2=holder_id, $3=acquired_at, $4=expires_at, $5=now (for expiry check).
const tryAcquireLeaseQuery = `
INSERT INTO leases (id, holder_id, acquired_at, expires_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (id) DO UPDATE SET
    holder_id   = EXCLUDED.holder_id,
    acquired_at = EXCLUDED.acquired_at,
    expires_at  = EXCLUDED.expires_at
WHERE leases.expires_at <= $5
RETURNING holder_id`

// releaseLeaseQuery removes the polling lease row for this holder only. A peer's
// row (different holder_id) is never deleted.
const releaseLeaseQuery = `DELETE FROM leases WHERE id = $1 AND holder_id = $2`

// leaseExistsQuery checks whether any lease row exists for the given id,
// returning the count (0 or 1). Used by Release to distinguish "already gone"
// from "held by a different holder".
const leaseExistsQuery = `SELECT COUNT(*) FROM leases WHERE id = $1`

// TryAcquire attempts to acquire the polling lease with the given TTL. It never
// returns a hard error for expected race outcomes (held by peer); those surface
// via the result struct. A database failure is carried on TransientErr.
func (s *PostgresLeaseStore) TryAcquire(ctx context.Context, ttl time.Duration) (LeaseAcquireResult, error) {
	now := s.now().UTC()
	expiresAt := now.Add(ttl)

	var holderID string
	err := s.db.QueryRow(ctx, tryAcquireLeaseQuery,
		leaseDocumentID, // $1 = id ("polling")
		s.holderID,      // $2 = holder_id
		now,             // $3 = acquired_at
		expiresAt,       // $4 = expires_at
		now,             // $5 = expiry check (WHERE leases.expires_at <= now)
	).Scan(&holderID)

	if errors.Is(err, pgx.ErrNoRows) {
		// The INSERT/UPDATE WHERE condition was false: a live lease is held by a peer.
		return LeaseAcquireResult{Held: true}, nil
	}
	if err != nil {
		return LeaseAcquireResult{TransientErr: fmt.Errorf("acquire polling lease: %w", err)}, nil
	}
	// The INSERT succeeded or the UPDATE replaced an expired row. The returned
	// holder_id is ours; store it in Handle.ETag for the conditional Release.
	return LeaseAcquireResult{Acquired: true, Handle: LeaseHandle{ETag: holderID}}, nil
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

	// rows_affected = 0: either the row is absent (already gone / expired) or it
	// is held by a different process. Check for the row's existence to distinguish.
	var count int
	if err := s.db.QueryRow(ctx, leaseExistsQuery, leaseDocumentID).Scan(&count); err != nil {
		return LeaseTransientError
	}
	if count == 0 {
		return LeaseAlreadyGone
	}
	return LeasePreconditionFailed
}
