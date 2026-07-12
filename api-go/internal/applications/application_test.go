package applications

import (
	"testing"
	"time"
)

// testApplication is a fully-populated planning application used across the
// package's tests.
func testApplication(t *testing.T) PlanningApplication {
	t.Helper()
	postcode := "EC2V 5AE"
	appType := "Full"
	appState := "Permitted"
	appSize := "Small"
	start := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	decided := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	consulted := time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC)
	lon := -0.0931
	lat := 51.5155
	url := "https://planit.example/app"
	link := "https://council.example/app"
	return PlanningApplication{
		Name:          "24/0123/FUL",
		UID:           "ABC-24-0123",
		AreaName:      "City of London",
		AreaID:        471,
		Address:       "1 Test Street",
		Postcode:      &postcode,
		Description:   "Single storey extension",
		AppType:       &appType,
		AppState:      &appState,
		AppSize:       &appSize,
		StartDate:     &start,
		DecidedDate:   &decided,
		ConsultedDate: &consulted,
		Longitude:     &lon,
		Latitude:      &lat,
		URL:           &url,
		Link:          &link,
		LastDifferent: time.Date(2026, 3, 2, 9, 30, 0, 0, time.UTC),
	}
}

func TestPlanningApplication_CanonicalUID(t *testing.T) {
	t.Parallel()
	a := PlanningApplication{Name: "24/0123/FUL", AreaID: 471}
	if got := a.CanonicalUID(); got != "471/24/0123/FUL" {
		t.Errorf("CanonicalUID: got %q, want 471/24/0123/FUL", got)
	}
}

func TestPlanningApplication_CanonicalUID_PreservesNameWithSlashes(t *testing.T) {
	t.Parallel()
	a := PlanningApplication{Name: "P/2026/0044/HH", AreaID: 9}
	if got := a.CanonicalUID(); got != "9/P/2026/0044/HH" {
		t.Errorf("CanonicalUID: got %q", got)
	}
}

func TestPlanningApplication_HasSameSilentFieldsAs(t *testing.T) {
	t.Parallel()
	base := testApplication(t)
	base.Reference = ptr("REF-1")
	base.Altid = []byte(`["A","B"]`)
	base.AssociatedID = []byte(`"single"`)
	base.ScraperName = ptr("scraper-1")
	base.OtherFields = map[string]any{"comment_url": "https://example.test/comment", "n_comments": float64(3)}
	base.LastChanged = timePtr(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	base.LastScraped = timePtr(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))

	t.Run("identical silent fields compare equal", func(t *testing.T) {
		t.Parallel()
		other := base
		if !base.HasSameSilentFieldsAs(other) {
			t.Error("identical silent fields must compare equal")
		}
	})

	t.Run("differing other_fields compares not-equal", func(t *testing.T) {
		t.Parallel()
		other := base
		other.OtherFields = map[string]any{"comment_url": "https://example.test/different", "n_comments": float64(3)}
		if base.HasSameSilentFieldsAs(other) {
			t.Error("a changed other_fields entry must compare not-equal")
		}
	})

	t.Run("equal maps built in different insertion order compare equal", func(t *testing.T) {
		t.Parallel()
		a := PlanningApplication{OtherFields: map[string]any{"a": 1.0, "b": 2.0, "c": 3.0}}
		b := PlanningApplication{OtherFields: map[string]any{"c": 3.0, "a": 1.0, "b": 2.0}}
		if !a.HasSameSilentFieldsAs(b) {
			t.Error("maps with the same entries in different insertion order must compare equal")
		}
	})

	t.Run("nil vs empty other_fields map compares equal", func(t *testing.T) {
		t.Parallel()
		a := PlanningApplication{OtherFields: nil}
		b := PlanningApplication{OtherFields: map[string]any{}}
		if !a.HasSameSilentFieldsAs(b) {
			t.Error("nil vs empty other_fields must compare equal")
		}
	})

	t.Run("differing reference compares not-equal", func(t *testing.T) {
		t.Parallel()
		other := base
		other.Reference = ptr("REF-2")
		if base.HasSameSilentFieldsAs(other) {
			t.Error("a changed reference must compare not-equal")
		}
	})

	t.Run("differing altid compares not-equal", func(t *testing.T) {
		t.Parallel()
		other := base
		other.Altid = []byte(`["A","C"]`)
		if base.HasSameSilentFieldsAs(other) {
			t.Error("a changed altid must compare not-equal")
		}
	})

	t.Run("differing associated_id compares not-equal", func(t *testing.T) {
		t.Parallel()
		other := base
		other.AssociatedID = []byte(`"other"`)
		if base.HasSameSilentFieldsAs(other) {
			t.Error("a changed associated_id must compare not-equal")
		}
	})

	t.Run("differing scraper_name compares not-equal", func(t *testing.T) {
		t.Parallel()
		other := base
		other.ScraperName = ptr("scraper-2")
		if base.HasSameSilentFieldsAs(other) {
			t.Error("a changed scraper_name must compare not-equal")
		}
	})

	t.Run("differing LastScraped alone compares equal (bookkeeping is ignored)", func(t *testing.T) {
		t.Parallel()
		other := base
		other.LastScraped = timePtr(base.LastScraped.Add(48 * time.Hour))
		if !base.HasSameSilentFieldsAs(other) {
			t.Error("a changed LastScraped alone must compare equal: it is bookkeeping, not silent")
		}
	})

	t.Run("differing LastChanged alone compares equal (bookkeeping is ignored)", func(t *testing.T) {
		t.Parallel()
		other := base
		other.LastChanged = timePtr(base.LastChanged.Add(48 * time.Hour))
		if !base.HasSameSilentFieldsAs(other) {
			t.Error("a changed LastChanged alone must compare equal: it is bookkeeping, not silent")
		}
	})

	t.Run("nil vs set altid compares not-equal", func(t *testing.T) {
		t.Parallel()
		other := base
		other.Altid = nil
		if base.HasSameSilentFieldsAs(other) {
			t.Error("nil vs set altid must compare not-equal")
		}
	})
}

// TestEqRawJSON_SemanticComparison proves eqRawJSON compares by decoded JSON
// value, not raw bytes: PlanIt's API returns pretty-printed JSON (preserved
// verbatim in json.RawMessage), while Postgres jsonb canonicalises text on
// storage (reformats spacing) — so for any record with an array-valued altid
// or associated_id, a bytes.Equal comparison would find the freshly-parsed
// incoming bytes and the read-back-from-Postgres bytes ALWAYS unequal, even
// when semantically identical. That permanently defeats the silent-change
// write-suppression guard for that subset of records, on every re-fetch —
// exactly the reindex-storm write amplification the three-bucket ingester
// design exists to prevent.
func TestEqRawJSON_SemanticComparison(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		a, b []byte
		want bool
	}{
		{
			name: "pretty-printed vs Postgres-canonical form of the same array compares equal",
			a:    []byte("[\"a\",\n  \"b\"]"),
			b:    []byte(`["a", "b"]`),
			want: true,
		},
		{
			name: "a string value vs a single-element array compares not-equal",
			a:    []byte(`"x"`),
			b:    []byte(`["x"]`),
			want: false,
		},
		{
			name: "nil vs nil compares equal",
			a:    nil,
			b:    nil,
			want: true,
		},
		{
			name: "genuinely different arrays compare not-equal",
			a:    []byte(`["a","b"]`),
			b:    []byte(`["a","c"]`),
			want: false,
		},
		{
			name: "unparseable bytes on either side compare not-equal (safe direction)",
			a:    []byte(`not-json`),
			b:    []byte(`["a"]`),
			want: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := eqRawJSON(tc.a, tc.b); got != tc.want {
				t.Errorf("eqRawJSON(%s, %s): got %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func ptr(s string) *string { return &s }

func timePtr(t time.Time) *time.Time { return &t }

func TestPlanningApplication_HasSameBusinessFieldsAs(t *testing.T) {
	t.Parallel()
	base := testApplication(t)

	// Same business fields, only LastDifferent bumped -> still equal (the
	// reindex-flood skip case the poll handler depends on).
	bumped := base
	bumped.LastDifferent = base.LastDifferent.Add(48 * time.Hour)
	if !base.HasSameBusinessFieldsAs(bumped) {
		t.Error("a bumped LastDifferent alone must compare equal")
	}

	// A changed business field (AppState) -> not equal.
	other := "Rejected"
	changed := base
	changed.AppState = &other
	if base.HasSameBusinessFieldsAs(changed) {
		t.Error("a changed AppState must compare not-equal")
	}

	// A changed coordinate -> not equal.
	newLon := *base.Longitude + 1
	movedLon := base
	movedLon.Longitude = &newLon
	if base.HasSameBusinessFieldsAs(movedLon) {
		t.Error("a changed longitude must compare not-equal")
	}

	// nil vs set pointer -> not equal.
	nilState := base
	nilState.AppState = nil
	if base.HasSameBusinessFieldsAs(nilState) {
		t.Error("nil vs set AppState must compare not-equal")
	}
}
