package webhooks

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateURL_Rejects(t *testing.T) {
	t.Parallel()
	const prefix = "https://hooks.example.com/"
	for _, tc := range []struct{ name, url string }{
		{"empty", ""},
		{"not parseable", "https://exa mple.com/%zz"},
		{"control character", "https://example.com/\x7f"},
		{"http scheme", "http://hooks.example.com/x"},
		{"no scheme", "hooks.example.com/x"},
		{"other scheme", "ftp://hooks.example.com/x"},
		{"opaque", "https:hooks.example.com"},
		{"empty host", "https:///x"},
		{"port without host", "https://:443/x"},
		{"userinfo", "https://user:pass@hooks.example.com/x"},
		{"empty userinfo", "https://@hooks.example.com/x"},
		{"other port", "https://hooks.example.com:8443/x"},
		{"port 80", "https://hooks.example.com:80/x"},
		{"too long", prefix + strings.Repeat("a", 2049-len(prefix))},
		{"loopback v4", "https://127.0.0.1/x"},
		{"private 10", "https://10.0.0.1/x"},
		{"private 172", "https://172.16.4.1/x"},
		{"private 192", "https://192.168.1.1/x"},
		{"azure metadata", "https://169.254.169.254/metadata"},
		{"cgnat", "https://100.100.1.57/x"},
		{"unspecified", "https://0.0.0.0/x"},
		{"this network", "https://0.1.2.3/x"},
		{"ietf protocol assignments", "https://192.0.0.8/x"},
		{"benchmarking", "https://198.19.0.1/x"},
		{"reserved", "https://240.0.0.1/x"},
		{"broadcast", "https://255.255.255.255/x"},
		{"multicast", "https://224.0.0.251/x"},
		{"loopback v6", "https://[::1]/x"},
		{"unspecified v6", "https://[::]/x"},
		{"unique local", "https://[fd00::1]/x"},
		{"link local v6", "https://[fe80::1]/x"},
		{"zoned", "https://[fe80::1%25en0]/x"},
		{"multicast v6", "https://[ff02::1]/x"},
		{"nat64", "https://[64:ff9b::a00:1]/x"},
		{"mapped loopback", "https://[::ffff:127.0.0.1]/x"},
		{"localhost", "https://localhost/x"},
		{"localhost upper case", "https://LocalHost/x"},
		{"localhost trailing dot", "https://localhost./x"},
		{"localhost subdomain", "https://a.localhost/x"},
		{"mdns", "https://printer.local/x"},
		{"internal", "https://metadata.google.internal/x"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateURL(tc.url)
			if !errors.Is(err, ErrInvalidURL) {
				t.Fatalf("ValidateURL = %v, want ErrInvalidURL", err)
			}
			if tc.url != "" && strings.Contains(err.Error(), tc.url) {
				t.Errorf("error %q carries the URL", err)
			}
		})
	}
}

func TestValidateURL_Accepts(t *testing.T) {
	t.Parallel()
	const prefix = "https://hooks.example.com/"
	for _, tc := range []struct{ name, url string }{
		{"plain", "https://hooks.example.com/abc"},
		{"explicit 443", "https://hooks.example.com:443/abc?x=1"},
		{"upper case scheme", "HTTPS://hooks.example.com/abc"},
		{"slack", "https://hooks.slack.com/services/T0/B0/token"},
		{"no path", "https://example.com"},
		{"public v4", "https://93.184.216.34/x"},
		{"public v6", "https://[2606:4700::1111]/x"},
		{"exactly 2048 bytes", prefix + strings.Repeat("a", 2048-len(prefix))},
		{"local as a label prefix", "https://local.example.com/x"},
		{"internal inside a label", "https://myinternal.com/x"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := ValidateURL(tc.url); err != nil {
				t.Errorf("ValidateURL = %v, want nil", err)
			}
		})
	}
}
