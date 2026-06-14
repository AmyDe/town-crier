package apns

import (
	"context"
	"encoding/json"
)

// NoOpSender is wired when APNs is disabled (no .p8 auth key configured), so a
// local-dev or push-less job boots cleanly without a real APNs endpoint. It
// drops every push and reports no invalid tokens.
type NoOpSender struct{}

// NewNoOpSender returns a sender that does nothing.
func NewNoOpSender() *NoOpSender { return &NoOpSender{} }

// Send discards the push and returns no invalid tokens.
func (NoOpSender) Send(context.Context, []string, json.RawMessage) ([]string, error) {
	return nil, nil
}
