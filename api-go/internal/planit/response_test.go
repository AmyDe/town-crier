package planit

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParsePlanItInstant(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		value   string
		want    time.Time
		wantErr bool
	}{
		{
			// PlanIt's real output: naive UTC, 6 fractional digits, no TZ.
			name:  "no-TZ six fractional digits",
			value: "2026-06-13T00:06:34.112581",
			want:  time.Date(2026, 6, 13, 0, 6, 34, 112581000, time.UTC),
		},
		{
			// PlanIt's real output: naive UTC, variable (4) fractional digits.
			name:  "no-TZ four fractional digits",
			value: "2026-06-13T01:33:50.2991",
			want:  time.Date(2026, 6, 13, 1, 33, 50, 299100000, time.UTC),
		},
		{
			name:  "no-TZ no fractional seconds",
			value: "2026-06-13T01:33:50",
			want:  time.Date(2026, 6, 13, 1, 33, 50, 0, time.UTC),
		},
		{
			name:  "TZ Zulu",
			value: "2026-06-13T00:06:34Z",
			want:  time.Date(2026, 6, 13, 0, 6, 34, 0, time.UTC),
		},
		{
			// A +01:00 offset must be normalised to the equivalent UTC instant.
			name:  "TZ positive offset normalises to UTC",
			value: "2026-06-13T00:06:34+01:00",
			want:  time.Date(2026, 6, 12, 23, 6, 34, 0, time.UTC),
		},
		{
			name:    "invalid value",
			value:   "not-a-time",
			wantErr: true,
		},
		{
			name:    "empty value",
			value:   "",
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parsePlanItInstant(tc.value)
			if (err != nil) != tc.wantErr {
				t.Fatalf("parsePlanItInstant(%q): got err=%v, wantErr=%v", tc.value, err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if !got.Equal(tc.want) {
				t.Errorf("parsePlanItInstant(%q): got %v, want %v", tc.value, got, tc.want)
			}
			if got.Location() != time.UTC {
				t.Errorf("parsePlanItInstant(%q): location = %v, want UTC", tc.value, got.Location())
			}
		})
	}
}

func TestToDomain_ParsesNoTimezoneFractionalLastDifferent(t *testing.T) {
	t.Parallel()
	rec := planItRecord{
		Name:          "24/0001",
		UID:           "24/0001/FUL",
		AreaName:      "Test",
		AreaID:        99,
		Address:       "1 High St",
		Description:   "A shed",
		AppType:       "Full",
		AppState:      "Undecided",
		LastDifferent: "2026-06-13T00:06:34.112581",
	}
	app, err := rec.toDomain()
	if err != nil {
		t.Fatalf("toDomain: %v", err)
	}
	want := time.Date(2026, 6, 13, 0, 6, 34, 112581000, time.UTC)
	if !app.LastDifferent.Equal(want) {
		t.Errorf("LastDifferent: got %v, want %v", app.LastDifferent, want)
	}
	if app.LastDifferent.Location() != time.UTC {
		t.Errorf("LastDifferent location = %v, want UTC", app.LastDifferent.Location())
	}
}

// richmondshireFixture mirrors the real 2026-07-12 Richmondshire record
// (GH#935): the 18 existing core fields, the six new top-level fields (altid
// and associated_id null in the real sample; reference also null), a 24-key
// other_fields map including the three DP-restricted keys (their real value is
// the literal placeholder "See source"), a redundant top-level GeoJSON
// location object (must be ignored — location_x/location_y already build the
// geography point), and the secs_taken/to envelope fields (also ignored).
const richmondshireFixture = `{
	"name": "24/0001",
	"uid": "24/0001/FUL",
	"area_name": "Richmondshire",
	"area_id": 350,
	"address": "1 High St",
	"postcode": "DL10 4AA",
	"description": "Erection of a shed",
	"app_type": "Full",
	"app_state": "Undecided",
	"app_size": "Small",
	"start_date": "2026-06-01",
	"location_x": -1.7297,
	"location_y": 54.4021,
	"url": "https://planit.example/app/1",
	"link": "https://council.example/app/1",
	"last_different": "2026-07-11T21:04:14.856483",
	"altid": null,
	"associated_id": null,
	"reference": null,
	"last_changed": "2026-07-11T21:04:14.856483",
	"last_scraped": "2026-07-11T20:00:00.123456",
	"scraper_name": "Richmondshire",
	"location": {"type": "Point", "coordinates": [-1.7297, 54.4021]},
	"secs_taken": 0.12,
	"to": 1,
	"other_fields": {
		"applicant_address": "1 High St, Richmond",
		"applicant_name": "See source",
		"agent_name": "See source",
		"application_type": "Full Planning Permission",
		"case_officer": "See source",
		"comment_date": "2026-07-25",
		"comment_url": "https://planit.example/comment/1",
		"date_received": "2026-06-01",
		"date_validated": "2026-06-05",
		"decided_by": "Delegated",
		"docs_url": "https://planit.example/docs/1",
		"easting": 417000,
		"lat": 54.4021,
		"lng": -1.7297,
		"map_url": "https://planit.example/map/1",
		"n_comments": 2,
		"n_constraints": 1,
		"n_documents": 5,
		"n_statutory_days": 56,
		"northing": 501000,
		"parish": "Richmond",
		"source_url": "https://richmondshire.example/app/1",
		"status": "Undecided",
		"target_decision_date": "2026-07-27",
		"ward_name": "Richmond Central"
	}
}`

func TestToDomain_FullFieldFixture_StripsRestrictedKeysAndKeepsTheRest(t *testing.T) {
	t.Parallel()
	var rec planItRecord
	if err := json.Unmarshal([]byte(richmondshireFixture), &rec); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	app, err := rec.toDomain()
	if err != nil {
		t.Fatalf("toDomain: %v", err)
	}

	for _, restricted := range []string{"applicant_name", "agent_name", "case_officer"} {
		if _, ok := app.OtherFields[restricted]; ok {
			t.Errorf("OtherFields must not contain restricted key %q", restricted)
		}
	}
	kept := []string{"comment_url", "comment_date", "docs_url", "n_dwellings", "ward_name", "applicant_address"}
	for _, key := range kept {
		if key == "n_dwellings" {
			continue // not present in this fixture; listed for reviewer clarity only
		}
		if _, ok := app.OtherFields[key]; !ok {
			t.Errorf("OtherFields must keep non-restricted key %q, got %+v", key, app.OtherFields)
		}
	}
	if got, want := app.OtherFields["comment_url"], "https://planit.example/comment/1"; got != want {
		t.Errorf("OtherFields[comment_url]: got %v, want %v", got, want)
	}

	if app.Reference != nil {
		t.Errorf("Reference: got %v, want nil", *app.Reference)
	}
	if app.Altid != nil {
		t.Errorf("Altid: got %s, want nil", app.Altid)
	}
	if app.AssociatedID != nil {
		t.Errorf("AssociatedID: got %s, want nil", app.AssociatedID)
	}
	if app.ScraperName == nil || *app.ScraperName != "Richmondshire" {
		t.Errorf("ScraperName: got %v, want Richmondshire", app.ScraperName)
	}
	wantChanged := time.Date(2026, 7, 11, 21, 4, 14, 856483000, time.UTC)
	if app.LastChanged == nil || !app.LastChanged.Equal(wantChanged) {
		t.Errorf("LastChanged: got %v, want %v", app.LastChanged, wantChanged)
	}
	wantScraped := time.Date(2026, 7, 11, 20, 0, 0, 123456000, time.UTC)
	if app.LastScraped == nil || !app.LastScraped.Equal(wantScraped) {
		t.Errorf("LastScraped: got %v, want %v", app.LastScraped, wantScraped)
	}
}

func TestToDomain_OtherFields_AbsentMapsToNil(t *testing.T) {
	t.Parallel()
	rec := planItRecord{
		Name: "24/0002", UID: "u", AreaName: "a", AreaID: 1, Address: "x",
		Description: "d", AppType: "t", AppState: "s", LastDifferent: "2026-06-13T00:06:34.112581",
	}
	app, err := rec.toDomain()
	if err != nil {
		t.Fatalf("toDomain: %v", err)
	}
	if app.OtherFields != nil {
		t.Errorf("OtherFields: got %v, want nil", app.OtherFields)
	}
}

func TestToDomain_OtherFields_EmptyAfterStrippingMapsToNil(t *testing.T) {
	t.Parallel()
	rec := planItRecord{
		Name: "24/0003", UID: "u", AreaName: "a", AreaID: 1, Address: "x",
		Description: "d", AppType: "t", AppState: "s", LastDifferent: "2026-06-13T00:06:34.112581",
		OtherFields: map[string]any{"applicant_name": "See source", "agent_name": "See source", "case_officer": "See source"},
	}
	app, err := rec.toDomain()
	if err != nil {
		t.Fatalf("toDomain: %v", err)
	}
	if app.OtherFields != nil {
		t.Errorf("OtherFields: got %v, want nil once only restricted keys remain", app.OtherFields)
	}
}

func TestToDomain_LastChangedLastScraped_LenientParsing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		value *string
	}{
		{"nil", nil},
		{"empty", ptrStr("")},
		{"malformed", ptrStr("not-a-timestamp")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := planItRecord{
				Name: "24/0004", UID: "u", AreaName: "a", AreaID: 1, Address: "x",
				Description: "d", AppType: "t", AppState: "s", LastDifferent: "2026-06-13T00:06:34.112581",
				LastChanged: tc.value, LastScraped: tc.value,
			}
			app, err := rec.toDomain()
			if err != nil {
				t.Fatalf("toDomain must never fail on a bookkeeping timestamp, got: %v", err)
			}
			if app.LastChanged != nil {
				t.Errorf("LastChanged: got %v, want nil", app.LastChanged)
			}
			if app.LastScraped != nil {
				t.Errorf("LastScraped: got %v, want nil", app.LastScraped)
			}
		})
	}
}

func TestToDomain_AltidAssociatedID_StringOrArrayShapeTolerance(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"string form", `"single-id"`, `"single-id"`},
		{"array form", `["a","b"]`, `["a","b"]`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := `{"name":"n","uid":"u","area_name":"a","area_id":1,"address":"x","description":"d",` +
				`"app_type":"t","app_state":"s","last_different":"2026-06-13T00:06:34.112581",` +
				`"altid":` + tc.raw + `,"associated_id":` + tc.raw + `}`
			var rec planItRecord
			if err := json.Unmarshal([]byte(body), &rec); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			app, err := rec.toDomain()
			if err != nil {
				t.Fatalf("toDomain: %v", err)
			}
			if string(app.Altid) != tc.want {
				t.Errorf("Altid: got %s, want %s", app.Altid, tc.want)
			}
			if string(app.AssociatedID) != tc.want {
				t.Errorf("AssociatedID: got %s, want %s", app.AssociatedID, tc.want)
			}
		})
	}
}

func ptrStr(s string) *string { return &s }
