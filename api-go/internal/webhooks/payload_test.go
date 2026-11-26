package webhooks

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
)

const (
	testEventID = "6f1d2c9e-8a7b-4c3d-9e0f-1a2b3c4d5e6f"
	testZoneID  = "c0ffee00-0000-4000-8000-000000000001"
)

func testApplication() applications.PlanningApplication {
	day := func(d int) *time.Time {
		t := time.Date(2026, 11, d, 0, 0, 0, 0, time.UTC)
		return &t
	}
	return applications.PlanningApplication{
		Name:        "26/01234/FUL",
		UID:         "leeds-uid",
		AreaName:    "Leeds",
		AreaID:      100,
		Address:     "1 High Street, Leeds",
		Postcode:    new("LS1 1AA"),
		Description: "Single-storey rear extension",
		AppType:     new("Full"),
		AppState:    new("Undecided"),
		AppSize:     new("Small"),
		StartDate:   day(20),
		DecidedDate: day(25),
		Latitude:    new(53.7997),
		Longitude:   new(-1.5492),
		URL:         new("https://publicaccess.leeds.gov.uk/app?id=1"),
	}
}

func testNotification(eventType notifications.EventType, zoneID *string, sources string) notifications.DigestNotification {
	return notifications.DigestNotification{
		ID:          "notification-1",
		UserID:      "auth0|private-user-id",
		WatchZoneID: zoneID,
		EventType:   eventType,
		Sources:     sources,
		CreatedAt:   time.Date(2026, 11, 26, 15, 0, 3, 999, time.FixedZone("x", 3600)),
	}
}

var (
	payloadKeys     = []string{"application", "eventId", "occurredAt", "schemaVersion", "sources", "type", "watchZone"}
	applicationKeys = []string{
		"address", "appSize", "appState", "appType", "authorityName", "authoritySlug", "councilUrl",
		"decidedDate", "description", "latitude", "longitude", "postcode", "reference", "shareUrl", "startDate",
	}
)

func marshalToMap(t *testing.T, p Payload) map[string]any {
	t.Helper()
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func object(t *testing.T, m map[string]any, key string) map[string]any {
	t.Helper()
	o, ok := m[key].(map[string]any)
	if !ok {
		t.Fatalf("%s = %#v, want an object", key, m[key])
	}
	return o
}

func assertKeys(t *testing.T, m map[string]any, want []string) {
	t.Helper()
	if got := slices.Sorted(maps.Keys(m)); !slices.Equal(got, want) {
		t.Errorf("keys = %v, want %v", got, want)
	}
}

func TestBuildPayload_NewApplication(t *testing.T) {
	t.Parallel()
	p, err := BuildPayload(testEventID, testNotification(notifications.EventNewApplication, new(testZoneID), "Zone"), testApplication(), "Home", "leeds")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"schemaVersion":1,"eventId":"6f1d2c9e-8a7b-4c3d-9e0f-1a2b3c4d5e6f","type":"application.new",` +
		`"occurredAt":"2026-11-26T14:00:03Z","watchZone":{"id":"c0ffee00-0000-4000-8000-000000000001","name":"Home"},` +
		`"sources":["zone"],"application":{"reference":"26/01234/FUL","authorityName":"Leeds","authoritySlug":"leeds",` +
		`"address":"1 High Street, Leeds","postcode":"LS1 1AA","description":"Single-storey rear extension",` +
		`"appType":"Full","appState":"Undecided","appSize":"Small","startDate":"2026-11-20","decidedDate":"2026-11-25",` +
		`"latitude":53.7997,"longitude":-1.5492,"councilUrl":"https://publicaccess.leeds.gov.uk/app?id=1",` +
		`"shareUrl":"https://share.towncrierapp.uk/a/leeds/26/01234/FUL"}}`
	var gotAny, wantAny any
	if err := json.Unmarshal(raw, &gotAny); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &wantAny); err != nil {
		t.Fatal(err)
	}
	gotJSON, _ := json.Marshal(gotAny)
	wantJSON, _ := json.Marshal(wantAny)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("payload\n got %s\nwant %s", gotJSON, wantJSON)
	}
}

func TestBuildPayload_KeySetAndNulls(t *testing.T) {
	t.Parallel()
	bare := applications.PlanningApplication{Name: "26/1", AreaName: "Leeds", AreaID: 100}
	nullable := []string{
		"appSize", "appState", "appType", "authoritySlug", "councilUrl", "decidedDate",
		"latitude", "longitude", "postcode", "shareUrl", "startDate",
	}
	newApp, err := BuildPayload("e1", testNotification(notifications.EventNewApplication, new(testZoneID), "Zone"), bare, "Home", "")
	if err != nil {
		t.Fatal(err)
	}
	zoneDecision, err := BuildPayload("e2", testNotification(notifications.EventDecisionUpdate, new(testZoneID), "Zone,Saved"), bare, "Home", "")
	if err != nil {
		t.Fatal(err)
	}
	savedDecision, err := BuildPayload("e3", testNotification(notifications.EventDecisionUpdate, nil, "Saved"), bare, "", "")
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name        string
		payload     Payload
		wantType    string
		wantZone    bool
		wantSources []any
		wantNull    []string
	}{
		{"new application", newApp, "application.new", true, []any{"zone"}, nullable},
		{"zone decision", zoneDecision, "application.decision", true, []any{"zone", "saved"}, nullable},
		{"saved-only decision", savedDecision, "application.decision", false, []any{"saved"}, nullable},
		{
			"test event", BuildTestPayload("e4", time.Date(2026, 11, 26, 14, 0, 0, 0, time.UTC)), "test", false, []any{},
			[]string{"shareUrl"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := marshalToMap(t, tc.payload)
			assertKeys(t, m, payloadKeys)
			app := object(t, m, "application")
			assertKeys(t, app, applicationKeys)
			if m["type"] != tc.wantType {
				t.Errorf("type = %v", m["type"])
			}
			if got, ok := m["sources"].([]any); !ok || !slices.Equal(got, tc.wantSources) {
				t.Errorf("sources = %#v, want %v", m["sources"], tc.wantSources)
			}
			if tc.wantZone {
				assertKeys(t, object(t, m, "watchZone"), []string{"id", "name"})
			} else if m["watchZone"] != nil {
				t.Errorf("watchZone = %v, want null", m["watchZone"])
			}
			for _, k := range tc.wantNull {
				if app[k] != nil {
					t.Errorf("application.%s = %v, want null", k, app[k])
				}
			}
		})
	}
}

func TestBuildPayload_ShareURLNeedsSlugAndReference(t *testing.T) {
	t.Parallel()
	app := testApplication()
	app.Name = ""
	p, err := BuildPayload("e", testNotification(notifications.EventNewApplication, new(testZoneID), "Zone"), app, "Home", "leeds")
	if err != nil {
		t.Fatal(err)
	}
	if p.Application.ShareURL != nil {
		t.Errorf("shareUrl = %q, want null", *p.Application.ShareURL)
	}
}

func TestBuildPayload_UnknownEventType(t *testing.T) {
	t.Parallel()
	_, err := BuildPayload("e", testNotification("Other", nil, "Zone"), testApplication(), "", "leeds")
	if err == nil {
		t.Fatal("want an error")
	}
}

func TestBuildPayload_NeverCarriesPrivateFields(t *testing.T) {
	t.Parallel()
	app := testApplication()
	app.OtherFields = map[string]any{"applicant_address": "OTHER-FIELD-VALUE"}
	app.Altid = []byte(`"ALTID-VALUE"`)
	app.AssociatedID = []byte(`"ASSOCIATED-VALUE"`)
	app.ScraperName = new("SCRAPER-VALUE")
	app.Link = new("https://www.planit.org.uk/planapplic/LINK-VALUE")
	app.Reference = new("PLANIT-REFERENCE-VALUE")
	app.UID = "UID-VALUE"
	n := testNotification(notifications.EventDecisionUpdate, new(testZoneID), "Zone")

	p, err := BuildPayload(testEventID, n, app, "Home", "leeds")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(raw))
	for _, banned := range []string{
		"otherfields", "other_fields", "altid", "associatedid", "scrapername", `"link"`, "userid", "email",
		"other-field-value", "altid-value", "associated-value", "scraper-value", "link-value",
		"planit-reference-value", "uid-value", "private-user-id", "notification-1",
	} {
		if strings.Contains(lower, banned) {
			t.Errorf("payload carries %q: %s", banned, raw)
		}
	}
}

func TestBuildTestPayload_FixedSample(t *testing.T) {
	t.Parallel()
	p := BuildTestPayload(testEventID, time.Date(2026, 11, 26, 15, 0, 0, 0, time.FixedZone("x", 3600)))
	if p.EventID != testEventID || p.OccurredAt != "2026-11-26T14:00:00Z" || p.SchemaVersion != 1 {
		t.Errorf("envelope = %+v", p)
	}
	a := p.Application
	if a.Reference != "TEST/0001" || a.AuthorityName != "Town Crier" || a.AuthoritySlug == nil || *a.AuthoritySlug != "test" || a.ShareURL != nil {
		t.Errorf("sample = %+v", a)
	}
}
