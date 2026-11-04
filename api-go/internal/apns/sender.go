package apns

import (
	"context"
	"encoding/json"
)

// PushSender delivers a pre-built APNs payload to device tokens and reports the
// tokens APNs permanently rejected so they can be pruned.
type PushSender interface {
	// Send delivers payload to each token, returning the tokens APNs reported as
	// permanently invalid. It never returns an error for a per-device failure —
	// those are logged and skipped; an error is reserved for a caller-level fault
	// (e.g. a cancelled context).
	Send(ctx context.Context, tokens []string, payload json.RawMessage) ([]string, error)
	// SendCollapsed is Send with an apns-collapse-id so a resend of the same
	// push replaces the first copy on the device. An empty collapseID behaves
	// exactly like Send.
	SendCollapsed(ctx context.Context, tokens []string, payload json.RawMessage, collapseID string) ([]string, error)
}
