package subscriptionevents

import (
	"crypto/rand"
	"fmt"
	"time"
)

// SchemaVersion is the payload schema version sent to webhooks.
const SchemaVersion = 1

// Payload is the anonymous JSON document POSTed to each webhook. It carries no
// user, transaction, product, offer-code or price data.
type Payload struct {
	SchemaVersion int          `json:"schemaVersion"`
	EventID       string       `json:"eventId"`
	Event         PayloadEvent `json:"event"`
	Totals        Totals       `json:"totals"`
}

// PayloadEvent describes the change itself.
type PayloadEvent struct {
	Type       EventType `json:"type"`
	FromTier   string    `json:"fromTier"`
	FromPlan   Plan      `json:"fromPlan"`
	ToTier     string    `json:"toTier"`
	ToPlan     Plan      `json:"toPlan"`
	Source     Source    `json:"source"`
	OccurredAt string    `json:"occurredAt"`
}

// BuildPayload assembles the wire payload for one classified change.
func BuildPayload(eventID string, typ EventType, before, after Snapshot, source Source, occurredAt time.Time, totals Totals) Payload {
	return Payload{
		SchemaVersion: SchemaVersion,
		EventID:       eventID,
		Event: PayloadEvent{
			Type:       typ,
			FromTier:   before.Tier.String(),
			FromPlan:   before.Plan,
			ToTier:     after.Tier.String(),
			ToPlan:     after.Plan,
			Source:     source,
			OccurredAt: occurredAt.UTC().Format(time.RFC3339),
		},
		Totals: totals,
	}
}

func newEventID() string {
	var b [16]byte
	rand.Read(b[:]) // never fails since Go 1.24; an unrecoverable failure crashes the process
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
