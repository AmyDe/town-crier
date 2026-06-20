package polling

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// leaseDocumentID is the single lease document gating concurrent poll cycles.
// The Leases container is partitioned by /id, so this lease lives in its own
// logical partition.
const leaseDocumentID = "polling"

// Lease-store sentinel errors. The CAS-aware Cosmos accessor returns these to
// distinguish the expected race outcomes from genuine transient failures. The
// store maps create/replace conflicts to Held and never lets them escape as
// errors.
var (
	// ErrLeaseConflict is returned by CreateLease when the document already exists
	// (Cosmos 409) — a peer created the lease first.
	ErrLeaseConflict = errors.New("lease already exists")
	// ErrLeasePreconditionFailed is returned by a CAS replace/delete when the etag
	// did not match (Cosmos 412) — a peer mutated the document first.
	ErrLeasePreconditionFailed = errors.New("lease precondition failed")
	// ErrLeaseNotFound is returned by DeleteLeaseWithETag when the document was
	// absent (Cosmos 404) — the lease had already expired or been released.
	ErrLeaseNotFound = errors.New("lease not found")
)

// leaseItems is the consumer-side slice of the Leases container the lease store
// needs: a CAS-aware read (returns the etag), a create-if-absent, an
// etag-conditional replace, and an etag-conditional delete. platform's CAS-aware
// container accessor satisfies it structurally.
type leaseItems interface {
	ReadLeaseWithETag(ctx context.Context, id string) (body []byte, etag string, found bool, err error)
	CreateLease(ctx context.Context, id string, item []byte) (etag string, err error)
	ReplaceLeaseWithETag(ctx context.Context, id string, item []byte, etag string) (newETag string, err error)
	DeleteLeaseWithETag(ctx context.Context, id, etag string) error
}

// LeaseHandle carries the etag of the winning write so Release can perform a
// conditional delete.
type LeaseHandle struct {
	ETag string
}

// LeaseAcquireResult is the outcome of a TryAcquire. Exactly one of Acquired,
// Held, or TransientErr-set is true.
type LeaseAcquireResult struct {
	// Acquired reports whether this caller now holds the lease; Handle is valid
	// only when Acquired.
	Acquired bool
	Handle   LeaseHandle
	// Held reports that a peer holds the lease (lost a create/replace race or the
	// existing lease is still live).
	Held bool
	// TransientErr is set when a network/5xx failure prevented a decision; the
	// caller should treat it as "could not acquire" and rely on the next trigger.
	TransientErr error
}

// LeaseReleaseOutcome is the outcome of a Release.
type LeaseReleaseOutcome int

const (
	// LeaseReleased means the lease document was deleted successfully.
	LeaseReleased LeaseReleaseOutcome = iota
	// LeaseAlreadyGone means the document was not found (already expired/released).
	LeaseAlreadyGone
	// LeasePreconditionFailed means the conditional delete failed (etag mismatch).
	LeasePreconditionFailed
	// LeaseTransientError means a network/5xx failure occurred; TTL is the backstop.
	LeaseTransientError
)

// leaseDocument is the Cosmos persistence shape for the polling lease. JSON tags
// use camelCase to match the stored document shape (holderId, acquiredAtUtc,
// expiresAtUtc). expiresAtUtc is the only field acquisition decisions consult;
// holderId and acquiredAtUtc are diagnostic. Times use the ISO 8601 round-trip
// layout with a numeric UTC offset.
type leaseDocument struct {
	ID            string `json:"id"`
	HolderID      string `json:"holderId"`
	AcquiredAtUTC string `json:"acquiredAtUtc"`
	ExpiresAtUTC  string `json:"expiresAtUtc"`
}

// LeaseStore is the etag-CAS-backed polling lease over the Cosmos Leases
// container. A missing document is created with create-if-absent; an expired
// document is replaced with an If-Match etag; a live document or a lost race
// yields Held. The lease prevents the orchestrator and the safety-net bootstrap
// from running a poll cycle concurrently.
type LeaseStore struct {
	items    leaseItems
	now      func() time.Time
	holderID string
}

// NewLeaseStore wires the lease store. now is injected so tests pin the clock;
// production passes time.Now. The holder id is a fresh random id per process,
// written as diagnostic metadata only — acquisition compares ExpiresAtUtc + etag.
func NewLeaseStore(items leaseItems, now func() time.Time) *LeaseStore {
	return &LeaseStore{
		items:    items,
		now:      now,
		holderID: newHolderID(),
	}
}

// TryAcquire attempts to acquire the polling lease with the given TTL. It never
// returns an error for the expected race outcomes (held by peer, raced on
// create/replace); those surface via the result. A transient failure is carried
// on TransientErr.
func (s *LeaseStore) TryAcquire(ctx context.Context, ttl time.Duration) (LeaseAcquireResult, error) {
	now := s.now().UTC()
	body, etag, found, err := s.items.ReadLeaseWithETag(ctx, leaseDocumentID)
	if err != nil {
		return LeaseAcquireResult{TransientErr: fmt.Errorf("read lease: %w", err)}, nil
	}

	desired, err := json.Marshal(leaseDocument{
		ID:            leaseDocumentID,
		HolderID:      s.holderID,
		AcquiredAtUTC: now.Format(dotNetRoundTrip),
		ExpiresAtUTC:  now.Add(ttl).Format(dotNetRoundTrip),
	})
	if err != nil {
		return LeaseAcquireResult{TransientErr: fmt.Errorf("encode lease: %w", err)}, nil
	}

	if !found {
		newETag, createErr := s.items.CreateLease(ctx, leaseDocumentID, desired)
		switch {
		case errors.Is(createErr, ErrLeaseConflict):
			return LeaseAcquireResult{Held: true}, nil
		case createErr != nil:
			return LeaseAcquireResult{TransientErr: fmt.Errorf("create lease: %w", createErr)}, nil
		default:
			return LeaseAcquireResult{Acquired: true, Handle: LeaseHandle{ETag: newETag}}, nil
		}
	}

	// A live (unexpired) lease is held by a peer; do not attempt to replace it.
	if expiresAt, perr := parseLeaseExpiry(body); perr == nil && expiresAt.After(now) {
		return LeaseAcquireResult{Held: true}, nil
	}

	// Expired (or unparseable expiry): replace under the read etag. A 412 means a
	// peer won the race.
	newETag, replaceErr := s.items.ReplaceLeaseWithETag(ctx, leaseDocumentID, desired, etag)
	switch {
	case errors.Is(replaceErr, ErrLeasePreconditionFailed):
		return LeaseAcquireResult{Held: true}, nil
	case replaceErr != nil:
		return LeaseAcquireResult{TransientErr: fmt.Errorf("replace lease: %w", replaceErr)}, nil
	default:
		return LeaseAcquireResult{Acquired: true, Handle: LeaseHandle{ETag: newETag}}, nil
	}
}

// Release performs a conditional delete using the handle's etag. It never returns
// an error — failures are surfaced as an outcome, and the lease TTL is the
// backstop for any non-Released outcome.
func (s *LeaseStore) Release(ctx context.Context, handle LeaseHandle) LeaseReleaseOutcome {
	err := s.items.DeleteLeaseWithETag(ctx, leaseDocumentID, handle.ETag)
	switch {
	case err == nil:
		return LeaseReleased
	case errors.Is(err, ErrLeaseNotFound):
		return LeaseAlreadyGone
	case errors.Is(err, ErrLeasePreconditionFailed):
		return LeasePreconditionFailed
	default:
		return LeaseTransientError
	}
}

// parseLeaseExpiry extracts the expiresAtUtc instant from a stored lease body.
func parseLeaseExpiry(body []byte) (time.Time, error) {
	var doc leaseDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return time.Time{}, fmt.Errorf("decode lease: %w", err)
	}
	t, err := time.Parse(time.RFC3339Nano, doc.ExpiresAtUTC)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse lease expiry %q: %w", doc.ExpiresAtUTC, err)
	}
	return t.UTC(), nil
}
