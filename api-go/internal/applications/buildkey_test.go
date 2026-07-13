package applications

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestBuildKeyMatches covers the exported matcher directly (GH#872 collateral,
// tc-zod82): it backs both requireBuildKey and the anonymous-rate-limit
// exemption predicate wired in cmd/api/wiring.go, so its empty-key behaviour
// must hold independent of either caller.
func TestBuildKeyMatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		expected  string
		provided  string
		setHeader bool
		want      bool
	}{
		{"matching key matches", "s3cret", "s3cret", true, true},
		{"wrong key does not match", "s3cret", "nope", true, false},
		{"absent header does not match", "s3cret", "", false, false},
		{"key prefix does not match (length-sensitive)", "s3cret", "s3cre", true, false},
		{"empty expected key never matches, even an empty header", "", "", true, false},
		{"empty expected key never matches a non-empty header", "", "anything", true, false},
		{"empty expected key never matches an absent header", "", "", false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/authorities/471/applications", nil)
			if tc.setHeader {
				req.Header.Set("X-Build-Key", tc.provided)
			}

			if got := BuildKeyMatches(req, tc.expected); got != tc.want {
				t.Errorf("BuildKeyMatches() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRequireBuildKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		expected   string
		provided   string
		setHeader  bool
		wantStatus int
		wantNext   bool
	}{
		{"matching key passes", "s3cret", "s3cret", true, http.StatusOK, true},
		{"wrong key rejected", "s3cret", "nope", true, http.StatusUnauthorized, false},
		{"missing header rejected", "s3cret", "", false, http.StatusUnauthorized, false},
		{"empty configured key rejects all", "", "anything", true, http.StatusUnauthorized, false},
		{"key prefix rejected (length-sensitive)", "s3cret", "s3cre", true, http.StatusUnauthorized, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			nextCalled := false
			h := requireBuildKey(tc.expected, func(w http.ResponseWriter, _ *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/authorities/471/applications", nil)
			if tc.setHeader {
				req.Header.Set("X-Build-Key", tc.provided)
			}
			rec := httptest.NewRecorder()
			h(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if nextCalled != tc.wantNext {
				t.Errorf("next called = %v, want %v", nextCalled, tc.wantNext)
			}
			if !tc.wantNext && rec.Body.Len() != 0 {
				t.Errorf("rejected body = %q, want empty (backfilled downstream)", rec.Body.String())
			}
		})
	}
}
