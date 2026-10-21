package applications

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestBuildKeyMatches covers the exported matcher directly (GH#872 collateral,
// tc-zod82): it backs the SEO handlers and the anonymous-rate-limit
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
