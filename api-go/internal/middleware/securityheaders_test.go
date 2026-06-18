package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders_SetsNoSniff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		innerCode  int
		wantStatus int
	}{
		{"200 success response", http.StatusOK, http.StatusOK},
		{"500 error response", http.StatusInternalServerError, http.StatusInternalServerError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.innerCode)
			})
			h := SecurityHeaders(inner)

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("X-Content-Type-Options = %q, want %q", got, "nosniff")
			}
			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}
