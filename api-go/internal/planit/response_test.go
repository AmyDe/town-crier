package planit

import (
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
