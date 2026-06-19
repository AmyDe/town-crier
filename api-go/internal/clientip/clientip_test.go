package clientip

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

// requestFrom builds a GET request with the given peer address (RemoteAddr) and
// optional headers, for driving FromRequest.
func requestFrom(t *testing.T, remoteAddr string, headers map[string]string) *http.Request {
	t.Helper()
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/me", nil)
	r.RemoteAddr = remoteAddr
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r
}

func TestFromRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		want       netip.Addr
	}{
		{
			name:       "cf ipv4 peer honours CF-Connecting-IP",
			remoteAddr: "104.16.0.1:443",
			headers:    map[string]string{"CF-Connecting-IP": "198.51.100.23"},
			want:       netip.MustParseAddr("198.51.100.23"),
		},
		{
			name:       "cf ipv6 peer honours CF-Connecting-IP",
			remoteAddr: "[2606:4700::1]:443",
			headers:    map[string]string{"CF-Connecting-IP": "198.51.100.23"},
			want:       netip.MustParseAddr("198.51.100.23"),
		},
		{
			name:       "cf peer honours an ipv6 CF-Connecting-IP",
			remoteAddr: "104.16.0.1:443",
			headers:    map[string]string{"CF-Connecting-IP": "2001:db8::99"},
			want:       netip.MustParseAddr("2001:db8::99"),
		},
		{
			name:       "ipv4-mapped ipv6 cf peer is recognised as cloudflare",
			remoteAddr: "[::ffff:104.16.0.1]:443",
			headers:    map[string]string{"CF-Connecting-IP": "198.51.100.23"},
			want:       netip.MustParseAddr("198.51.100.23"),
		},
		{
			name:       "non-cf peer ignores spoofed CF-Connecting-IP",
			remoteAddr: "203.0.113.5:51000",
			headers:    map[string]string{"CF-Connecting-IP": "198.51.100.23"},
			want:       netip.MustParseAddr("203.0.113.5"),
		},
		{
			name:       "cf peer with no CF-Connecting-IP falls back to peer",
			remoteAddr: "104.16.0.1:443",
			want:       netip.MustParseAddr("104.16.0.1"),
		},
		{
			name:       "cf peer with unparseable CF-Connecting-IP falls back to peer",
			remoteAddr: "104.16.0.1:443",
			headers:    map[string]string{"CF-Connecting-IP": "not-an-ip"},
			want:       netip.MustParseAddr("104.16.0.1"),
		},
		{
			name:       "X-Forwarded-For is never trusted from a cf peer",
			remoteAddr: "104.16.0.1:443",
			headers:    map[string]string{"X-Forwarded-For": "9.9.9.9"},
			want:       netip.MustParseAddr("104.16.0.1"),
		},
		{
			name:       "X-Forwarded-For is never trusted from a non-cf peer",
			remoteAddr: "203.0.113.5:51000",
			headers:    map[string]string{"X-Forwarded-For": "9.9.9.9"},
			want:       netip.MustParseAddr("203.0.113.5"),
		},
		{
			name:       "ipv6 non-cf peer returns the peer",
			remoteAddr: "[2001:db8::1]:51000",
			want:       netip.MustParseAddr("2001:db8::1"),
		},
		{
			name:       "remote addr without a port still parses",
			remoteAddr: "203.0.113.5",
			want:       netip.MustParseAddr("203.0.113.5"),
		},
		{
			name:       "unparseable remote addr yields the zero addr",
			remoteAddr: "garbage",
			want:       netip.Addr{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := FromRequest(requestFrom(t, tc.remoteAddr, tc.headers))
			if got != tc.want {
				t.Errorf("FromRequest() = %v, want %v", got, tc.want)
			}
		})
	}
}
