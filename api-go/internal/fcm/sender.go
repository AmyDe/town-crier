package fcm

import (
	"context"
	"encoding/json"
)

// PushSender is the contract the platform-aware dispatcher consumes to deliver a
// pre-built FCM message body to a set of Android device tokens, learning which
// tokens FCM has permanently rejected so they can be pruned. It mirrors
// apns.PushSender exactly (same signature) so the dispatcher can treat both
// platforms uniformly.
//
// The payload is the FCM v1 "message" object WITHOUT the per-recipient token —
// the Client injects each token itself, since v1 has no multicast and posts one
// request per device.
type PushSender interface {
	// Send delivers payload to each token, returning the tokens FCM reported as
	// permanently invalid (UNREGISTERED / INVALID_ARGUMENT). It never returns an
	// error for a per-device failure — those are logged and skipped; an error is
	// reserved for a caller-level fault (e.g. a cancelled context).
	Send(ctx context.Context, tokens []string, payload json.RawMessage) ([]string, error)
}
