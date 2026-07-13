package acsemail

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// tracerName labels the "Email send" wrapper span this package emits.
const tracerName = "github.com/AmyDe/town-crier/api-go/internal/acsemail"

// InstrumentedSender wraps an EmailSender in exactly one "Email send" span per
// call, tagged email.kind, so a per-email dependency is visible in App
// Insights distinct from the transport-level "ACS email send" HTTP client
// spans a single logical send can emit (one POST plus, when the operation
// doesn't complete synchronously, one or more status polls — inflating the
// raw per-email span count). It never renames or alters those wrapped spans;
// it only adds a span around the call.
type InstrumentedSender struct {
	next EmailSender
}

// NewInstrumentedSender wraps next. *Client and NoOpSender both satisfy
// EmailSender.
func NewInstrumentedSender(next EmailSender) *InstrumentedSender {
	return &InstrumentedSender{next: next}
}

// Send starts the "Email send" wrapper span tagged with kind, delegates to
// the wrapped sender, and — on failure — records the error and Error status
// on the span before returning it, so the dependency's success flag reflects
// the outcome.
func (s *InstrumentedSender) Send(ctx context.Context, kind string, msg Message) error {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "Email send")
	defer span.End()
	span.SetAttributes(attribute.String("email.kind", kind))

	if err := s.next.Send(ctx, msg); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}
