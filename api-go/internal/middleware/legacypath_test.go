package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTraceFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"legacy token path is dropped", "/v1/me/device-token/abc123", false},
		{"legacy token path with escapes is dropped", "/v1/me/device-token/a%2Fb", false},
		{"body route is traced", "/v1/me/device-token", true},
		{"other route is traced", "/v1/me/profile", true},
		{"double slash variant is dropped", "//v1/me/device-token/abc123", false},
		{"dot segment variant is dropped", "/v1/me/./device-token/abc123", false},
		{"upper case variant is dropped", "/V1/ME/Device-Token/abc123", false},
		{"similar prefix is traced", "/v1/me/device-tokens", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, tt.path, nil)
			if got := TraceFilter(r); got != tt.want {
				t.Errorf("TraceFilter(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestRedactPath(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, in, want string }{
		{"double slash variant", "//v1/me/device-token/abc123", "//v1/me/device-token/{redacted}"},
		{"dot segment variant", "/v1/me/./device-token/abc123/x", "/v1/me/./device-token/{redacted}"},
		{"upper case variant", "/V1/ME/Device-Token/abc123", "/V1/ME/Device-Token/{redacted}"},
		{"legacy token path", "/v1/me/device-token/abc123", "/v1/me/device-token/{redacted}"},
		{"body route", "/v1/me/device-token", "/v1/me/device-token"},
		{"other route", "/v1/me/profile", "/v1/me/profile"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := RedactPath(tt.in); got != tt.want {
				t.Errorf("RedactPath(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestTraceFilter_RawPath(t *testing.T) {
	t.Parallel()

	r := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/v1/me/device-token/abc", nil)
	r.URL.Path = "/v1/me/other"
	r.URL.RawPath = "/v1/me/%64evice-token/abc"
	if TraceFilter(r) {
		t.Error("TraceFilter should drop a request whose RawPath carries the segment")
	}

	r = httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/v1/me/device-token", nil)
	if !TraceFilter(r) {
		t.Error("TraceFilter should keep the body route")
	}
}
