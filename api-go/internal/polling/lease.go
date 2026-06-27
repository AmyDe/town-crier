package polling

// leaseDocumentID is the id of the single lease row gating concurrent poll
// cycles. It lives in its own row keyed by id='polling'.
const leaseDocumentID = "polling"

// LeaseHandle carries the holder id of the winning write so Release can perform a
// conditional (holder-scoped) delete.
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
	// Held reports that a peer holds the lease (lost an acquire race or the
	// existing lease is still live).
	Held bool
	// TransientErr is set when a database/network failure prevented a decision; the
	// caller should treat it as "could not acquire" and rely on the next trigger.
	TransientErr error
}

// LeaseReleaseOutcome is the outcome of a Release.
type LeaseReleaseOutcome int

const (
	// LeaseReleased means the lease row was deleted successfully.
	LeaseReleased LeaseReleaseOutcome = iota
	// LeaseAlreadyGone means the row was not found (already expired/released).
	LeaseAlreadyGone
	// LeasePreconditionFailed means the conditional delete failed (held by a peer).
	LeasePreconditionFailed
	// LeaseTransientError means a database/network failure occurred; TTL is the backstop.
	LeaseTransientError
)
