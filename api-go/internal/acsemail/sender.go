package acsemail

import "context"

// EmailSender delivers a pre-rendered email.
type EmailSender interface {
	// Send delivers msg, returning an error when ACS rejects the request or the
	// send operation finishes in a non-success state.
	Send(ctx context.Context, msg Message) error
}
