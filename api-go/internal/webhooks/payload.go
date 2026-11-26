package webhooks

import (
	"fmt"
	"strings"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/sharelink"
)

// SchemaVersion is the version of the JSON payload.
const SchemaVersion = 1

// EventType is the public type of a webhook event.
type EventType string

// EventType values. EventTest is sent only by the test endpoint.
const (
	EventApplicationNew      EventType = "application.new"
	EventApplicationDecision EventType = "application.decision"
	EventTest                EventType = "test"
)

// Payload is the schemaVersion 1 wire payload of one event. Its key set is a
// public contract: it carries one alert and no account data.
type Payload struct {
	SchemaVersion int                `json:"schemaVersion"`
	EventID       string             `json:"eventId"`
	Type          EventType          `json:"type"`
	OccurredAt    string             `json:"occurredAt"`
	WatchZone     *PayloadZone       `json:"watchZone"`
	Sources       []string           `json:"sources"`
	Application   PayloadApplication `json:"application"`
}

// PayloadZone is the watch zone that matched. It is null for a decision on a
// saved application only.
type PayloadZone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// PayloadApplication is the planning application of an event. A nil field
// serialises as null and is never omitted.
type PayloadApplication struct {
	Reference     string             `json:"reference"`
	AuthorityName string             `json:"authorityName"`
	AuthoritySlug *string            `json:"authoritySlug"`
	Address       string             `json:"address"`
	Postcode      *string            `json:"postcode"`
	Description   string             `json:"description"`
	AppType       *string            `json:"appType"`
	AppState      *string            `json:"appState"`
	AppSize       *string            `json:"appSize"`
	StartDate     *platform.DateOnly `json:"startDate"`
	DecidedDate   *platform.DateOnly `json:"decidedDate"`
	Latitude      *float64           `json:"latitude"`
	Longitude     *float64           `json:"longitude"`
	CouncilURL    *string            `json:"councilUrl"`
	ShareURL      *string            `json:"shareUrl"`
}

// BuildPayload builds the payload for one notification record. zoneName is used
// only when the record has a watch zone. authoritySlug is "" when the
// authority is unknown. It fails for an event type that has no public form.
func BuildPayload(eventID string, n notifications.DigestNotification, app applications.PlanningApplication, zoneName, authoritySlug string) (Payload, error) {
	typ, err := publicEventType(n.EventType)
	if err != nil {
		return Payload{}, err
	}
	var zone *PayloadZone
	if n.WatchZoneID != nil && *n.WatchZoneID != "" {
		zone = &PayloadZone{ID: *n.WatchZoneID, Name: zoneName}
	}
	return Payload{
		SchemaVersion: SchemaVersion,
		EventID:       eventID,
		Type:          typ,
		OccurredAt:    formatInstant(n.CreatedAt),
		WatchZone:     zone,
		Sources:       splitSources(n.Sources),
		Application: PayloadApplication{
			Reference:     app.Name,
			AuthorityName: app.AreaName,
			AuthoritySlug: nonEmpty(authoritySlug),
			Address:       app.Address,
			Postcode:      app.Postcode,
			Description:   app.Description,
			AppType:       app.AppType,
			AppState:      app.AppState,
			AppSize:       app.AppSize,
			StartDate:     platform.DateOnlyPtr(app.StartDate),
			DecidedDate:   platform.DateOnlyPtr(app.DecidedDate),
			Latitude:      app.Latitude,
			Longitude:     app.Longitude,
			CouncilURL:    app.URL,
			ShareURL:      nonEmpty(sharelink.ApplicationURL(authoritySlug, app.Name)),
		},
	}, nil
}

// BuildTestPayload builds the test event: a fixed sample application with no
// watch zone and no share URL.
func BuildTestPayload(eventID string, now time.Time) Payload {
	return Payload{
		SchemaVersion: SchemaVersion,
		EventID:       eventID,
		Type:          EventTest,
		OccurredAt:    formatInstant(now),
		Sources:       []string{},
		Application: PayloadApplication{
			Reference:     "TEST/0001",
			AuthorityName: "Town Crier",
			AuthoritySlug: new("test"),
			Address:       "1 Example Street, Testtown",
			Postcode:      new("ZZ1 1ZZ"),
			Description:   "A made-up planning application, sent to test your webhook",
			AppType:       new("Full"),
			AppState:      new("Undecided"),
			AppSize:       new("Small"),
			StartDate:     platform.DateOnlyPtr(new(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))),
			DecidedDate:   platform.DateOnlyPtr(new(time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC))),
			Latitude:      new(51.5074),
			Longitude:     new(-0.1278),
			CouncilURL:    new("https://towncrierapp.uk/developers/webhooks"),
		},
	}
}

func publicEventType(t notifications.EventType) (EventType, error) {
	switch t {
	case notifications.EventNewApplication:
		return EventApplicationNew, nil
	case notifications.EventDecisionUpdate:
		return EventApplicationDecision, nil
	default:
		return "", fmt.Errorf("webhooks: no payload type for notification event %q", t)
	}
}

func formatInstant(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func splitSources(sources string) []string {
	out := []string{}
	for part := range strings.SplitSeq(sources, ",") {
		if s := strings.ToLower(strings.TrimSpace(part)); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
