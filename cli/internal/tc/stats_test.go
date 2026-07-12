package tc

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// sampleStats builds a fully-populated statsResponse for render assertions.
func sampleStats() *statsResponse {
	return &statsResponse{
		Users: statsUsers{
			Total:  42,
			ByTier: statsByTier{Free: 30, Personal: 8, Pro: 4},
		},
		Paying: statsPaying{
			EffectivePaid:  12,
			AppStore:       9,
			Comped:         3,
			Lapsed:         2,
			InGrace:        1,
			AppStoreByTier: &statsAppStoreByTier{Personal: 3, Pro: 6},
		},
		Signups: statsSignups{
			Last24h:    3,
			Last7d:     11,
			Last30d:    28,
			MostRecent: &statsMostRecent{UserID: "auth0|u1", Email: strptr("alice@example.com"), CreatedAt: "2026-07-01T09:00:00Z"},
		},
		Activity: statsActivity{
			Active24h:      5,
			Active7d:       20,
			ZeroWatchZones: 7,
			NoEmail:        2,
		},
		Reach: statsReach{
			WatchZones:          88,
			SavedApplications:   150,
			DeviceRegistrations: 40,
			NotificationsSent:   500,
			NotificationsUnread: 45,
		},
	}
}

// TestRenderStats_ContainsAggregates asserts the compact block groups every
// metric under its five headings with the right values.
func TestRenderStats_ContainsAggregates(t *testing.T) {
	t.Parallel()
	var sb strings.Builder
	renderStats(&sb, sampleStats())
	out := sb.String()

	for _, want := range []string{
		// Headings.
		"Users", "Paying", "Signups", "Activity", "Reach",
		// Users.
		"Total: 42", "Free 30, Personal 8, Pro 4",
		// Paying: App Store headline with tier split, estimated MRR, then the
		// non-App-Store buckets. "Effective paid" is gone — the headline paying
		// number is App Store only.
		"Paying (App Store): 9 (Personal 3, Pro 6)", "Est. MRR: £35.91/mo",
		"Comped (offer/admin): 3", "Lapsed: 2", "In grace: 1",
		// Signups.
		"Last 24h: 3", "Last 7d: 11", "Last 30d: 28",
		"auth0|u1 (alice@example.com) at 2026-07-01T09:00:00Z",
		// Activity.
		"Active 24h: 5", "Active 7d: 20", "Zero watch zones: 7", "No email: 2",
		// Reach.
		"Watch zones: 88", "Saved applications: 150", "Device registrations: 40",
		"Notifications sent: 500", "Notifications unread: 45",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("render missing %q:\n%s", want, out)
		}
	}
}

// TestRenderStats_NullMostRecentAndEmail covers the two null-degradation paths:
// a nil mostRecent (empty user base) and a non-nil mostRecent with a nil email.
func TestRenderStats_NullMostRecentAndEmail(t *testing.T) {
	t.Parallel()

	t.Run("nil most recent", func(t *testing.T) {
		t.Parallel()
		s := sampleStats()
		s.Signups.MostRecent = nil
		var sb strings.Builder
		renderStats(&sb, s)
		if !strings.Contains(sb.String(), "Most recent: (none)") {
			t.Errorf("nil mostRecent should render (none):\n%s", sb.String())
		}
	})

	t.Run("nil email", func(t *testing.T) {
		t.Parallel()
		s := sampleStats()
		s.Signups.MostRecent = &statsMostRecent{UserID: "apple|u9", Email: nil, CreatedAt: "2026-07-01T10:00:00Z"}
		var sb strings.Builder
		renderStats(&sb, s)
		if !strings.Contains(sb.String(), "apple|u9 (none) at 2026-07-01T10:00:00Z") {
			t.Errorf("nil email should render (none):\n%s", sb.String())
		}
	})
}

// TestRenderStats_NilAppStoreByTier_Degrades covers an older API build that
// predates the tier split: the Paying block must still render the App
// Store-only headline (no parenthesised split) and "Est. MRR: -", never
// panic on the nil pointer.
func TestRenderStats_NilAppStoreByTier_Degrades(t *testing.T) {
	t.Parallel()
	s := sampleStats()
	s.Paying.AppStoreByTier = nil
	var sb strings.Builder
	renderStats(&sb, s)
	out := sb.String()
	if !strings.Contains(out, "Paying (App Store): 9\n") {
		t.Errorf("nil AppStoreByTier should render the bare App Store count:\n%s", out)
	}
	if strings.Contains(out, "(Personal") {
		t.Errorf("nil AppStoreByTier must not render a tier split:\n%s", out)
	}
	if !strings.Contains(out, "Est. MRR: -") {
		t.Errorf("nil AppStoreByTier should render Est. MRR: -:\n%s", out)
	}
}

// TestRenderStats_ZeroPayers covers a populated-but-empty AppStoreByTier (a
// real API response with no App Store payers yet): the MRR line must still
// render a concrete amount, £0.00/mo, not the nil-degradation dash.
func TestRenderStats_ZeroPayers(t *testing.T) {
	t.Parallel()
	s := sampleStats()
	s.Paying.AppStore = 0
	s.Paying.AppStoreByTier = &statsAppStoreByTier{Personal: 0, Pro: 0}
	var sb strings.Builder
	renderStats(&sb, s)
	out := sb.String()
	if !strings.Contains(out, "Paying (App Store): 0 (Personal 0, Pro 0)") {
		t.Errorf("zero payers should still render the tier split:\n%s", out)
	}
	if !strings.Contains(out, "Est. MRR: £0.00/mo") {
		t.Errorf("zero payers should render Est. MRR: £0.00/mo:\n%s", out)
	}
}

// TestMRRPence covers the integer-pence MRR arithmetic directly: Pro payers at
// 499p/mo, Personal payers at 199p/mo, no floats anywhere near the money.
func TestMRRPence(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		tier *statsAppStoreByTier
		want int
	}{
		{"nil tier", nil, 0},
		{"zero payers", &statsAppStoreByTier{Personal: 0, Pro: 0}, 0},
		{"2 pro + 3 personal", &statsAppStoreByTier{Personal: 3, Pro: 2}, 2*499 + 3*199},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := mrrPence(tc.tier); got != tc.want {
				t.Errorf("mrrPence(%+v) = %d, want %d", tc.tier, got, tc.want)
			}
		})
	}
}

// TestFormatMRR covers the pence-to-"£X.YY/mo" rendering, including the
// zero-payer case (a real, formatted £0.00/mo — not the nil-degradation dash).
func TestFormatMRR(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		tier *statsAppStoreByTier
		want string
	}{
		{"zero payers", &statsAppStoreByTier{}, "£0.00/mo"},
		{"2 pro + 3 personal", &statsAppStoreByTier{Personal: 3, Pro: 2}, "£15.95/mo"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := formatMRR(tc.tier); got != tc.want {
				t.Errorf("formatMRR(%+v) = %q, want %q", tc.tier, got, tc.want)
			}
		})
	}
}

// TestStatsSummaryLine renders the one-line list-users header from the aggregate.
func TestStatsSummaryLine(t *testing.T) {
	t.Parallel()
	got := statsSummaryLine(sampleStats())
	for _, want := range []string{
		"42 users", "Free 30", "Personal 8", "Pro 4",
		"paying 9", "MRR £35.91/mo", "comped 3", "lapsed 2",
		"new 24h 3", "active 24h 5",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("summary line missing %q:\n%s", want, got)
		}
	}
}

// TestStatsSummaryLine_NilAppStoreByTier_Degrades covers the same older-API
// nil case for the one-line list-users header: the paying (App Store) count
// is unaffected (it was never part of the split), but the MRR segment
// degrades to "MRR -" rather than computing a wrong figure from no data.
func TestStatsSummaryLine_NilAppStoreByTier_Degrades(t *testing.T) {
	t.Parallel()
	s := sampleStats()
	s.Paying.AppStoreByTier = nil
	got := statsSummaryLine(s)
	if !strings.Contains(got, "paying 9") {
		t.Errorf("summary line missing paying 9:\n%s", got)
	}
	if !strings.Contains(got, "MRR -") {
		t.Errorf("summary line should degrade to MRR -:\n%s", got)
	}
	if strings.Contains(got, "£") {
		t.Errorf("summary line must not render a computed amount when nil:\n%s", got)
	}
}

// TestRunStats_SuccessRendersBlock drives runStats against a fake endpoint and
// asserts it GETs the pinned path, sends the admin key, and renders the block.
func TestRunStats_SuccessRendersBlock(t *testing.T) {
	t.Parallel()
	var gotPath, gotMethod, gotKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotKey = r.Header.Get(apiKeyHeader)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, testStatsJSON)
	}))
	defer server.Close()

	env, out, errBuf := captureEnv()
	code := runStats(context.Background(), clientFor(server), env, ParseArgs([]string{"stats"}))
	if code != exitOK {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, errBuf.String())
	}
	if gotMethod != http.MethodGet || gotPath != "/v1/admin/stats" {
		t.Fatalf("request = %s %s, want GET /v1/admin/stats", gotMethod, gotPath)
	}
	if gotKey != "sk-test" {
		t.Fatalf("X-Admin-Key = %q, want sk-test", gotKey)
	}
	if got := out.String(); !strings.Contains(got, "Total: 2") || !strings.Contains(got, "Paying") {
		t.Fatalf("stdout missing rendered stats:\n%s", got)
	}
}

// TestRunStats_APIErrorReturns2 mirrors runListUsers' non-2xx handling.
func TestRunStats_APIErrorReturns2(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "boom")
	}))
	defer server.Close()

	env, _, errBuf := captureEnv()
	code := runStats(context.Background(), clientFor(server), env, ParseArgs([]string{"stats"}))
	if code != exitRuntime {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "API error (500): boom") {
		t.Fatalf("stderr = %q, want API error (500)", errBuf.String())
	}
}
