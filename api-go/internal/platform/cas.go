package platform

import "errors"

// ErrCASPreconditionFailed is the shared optimistic-concurrency sentinel: a
// conditional (compare-and-swap) write lost its race because the underlying row
// changed between read and write. The Postgres stores wrap their version-mismatch
// failures as this error so the watch-zone and offer-code quota CAS handlers can
// branch on a single, store-agnostic sentinel.
var ErrCASPreconditionFailed = errors.New("precondition failed")
