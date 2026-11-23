package polling

import (
	"crypto/rand"
	"encoding/hex"
)

// newHolderID returns a random hex id for this process's lease writes, so
// concurrent pollers never share a holder identity.
func newHolderID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand.Read never fails on supported platforms, and the id is only
		// diagnostic, so fall back to a constant rather than panic.
		return "unknown-holder"
	}
	return hex.EncodeToString(b[:])
}
