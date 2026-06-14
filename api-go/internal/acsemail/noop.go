package acsemail

import "context"

// NoOpSender is wired when no ACS connection string is configured, so a local
// dev or email-less job boots cleanly without ACS credentials. It drops every
// email.
type NoOpSender struct{}

// NewNoOpSender returns a sender that does nothing.
func NewNoOpSender() *NoOpSender { return &NoOpSender{} }

// Send discards the message.
func (NoOpSender) Send(context.Context, Message) error { return nil }
