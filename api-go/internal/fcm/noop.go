package fcm

import (
	"context"
	"encoding/json"
)

// NoOpSender is wired when FCM is disabled (no service-account JSON configured),
// so a worker job boots cleanly without a real FCM endpoint. It drops every push
// and reports no invalid tokens — the mirror of apns.NoOpSender.
type NoOpSender struct{}

// NewNoOpSender returns a sender that does nothing.
func NewNoOpSender() *NoOpSender { return &NoOpSender{} }

// Send discards the push and returns no invalid tokens.
func (NoOpSender) Send(context.Context, []string, json.RawMessage) ([]string, error) {
	return nil, nil
}
