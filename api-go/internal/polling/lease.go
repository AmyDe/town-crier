package polling

// leaseDocumentID is the id of the single lease row gating concurrent poll
// cycles.
const leaseDocumentID = "polling"

// LeaseHandle carries the winning holder id in ETag so Release can perform a
// holder-scoped delete.
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
