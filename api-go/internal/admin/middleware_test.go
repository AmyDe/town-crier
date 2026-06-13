package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireAdminKey(t *testing.T) {
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
			h := requireAdminKey(tc.expected, func(w http.ResponseWriter, _ *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/admin/users", nil)
			if tc.setHeader {
				req.Header.Set("X-Admin-Key", tc.provided)
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
