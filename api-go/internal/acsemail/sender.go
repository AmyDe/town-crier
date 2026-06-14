package acsemail

import "context"

// EmailSender is the contract the digest worker (tc-34y5) consumes to deliver a
// pre-rendered email. It is exported here because the consuming bead lands later
// and needs a stable name to wire either the real Client or the NoOpSender
// behind; the constructors return concrete structs, the interface exists for the
// consumer to depend on.
type EmailSender interface {
	// Send delivers msg, returning an error when ACS rejects the request or the
	// send operation finishes in a non-success state.
	Send(ctx context.Context, msg Message) error
}
