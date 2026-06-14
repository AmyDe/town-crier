package apns

import (
	"context"
	"encoding/json"
)

// PushSender is the contract the digest worker (tc-34y5) consumes to deliver a
// pre-built APNs payload to a set of device tokens, learning which tokens APNs
// has permanently rejected so they can be pruned.
//
// Idiomatic Go defines an interface at its consumer; this one is exported here
// because the consuming bead lands later and needs a stable name to wire either
// the real Client or the NoOpSender behind. Both are concrete structs returned
// by their constructors — the interface exists only for the consumer to depend
// on, not as a constructor return type.
type PushSender interface {
	// Send delivers payload to each token, returning the tokens APNs reported as
	// permanently invalid. It never returns an error for a per-device failure —
	// those are logged and skipped; an error is reserved for a caller-level fault
	// (e.g. a cancelled context).
	Send(ctx context.Context, tokens []string, payload json.RawMessage) ([]string, error)
}
