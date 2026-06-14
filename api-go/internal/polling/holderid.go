package polling

import (
	"crypto/rand"
	"encoding/hex"
)

// newHolderID returns a fresh random hex id for this process's lease writes. It
// is diagnostic metadata only (acquisition decisions compare expiry + etag), but
// crypto/rand is used so two concurrent pollers never collide on holder identity.
func newHolderID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand.Read never fails on supported platforms; fall back to a
		// constant rather than panicking in library code — the id is diagnostic.
		return "unknown-holder"
	}
	return hex.EncodeToString(b[:])
}
