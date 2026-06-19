// Package clientip resolves the real client IP for an inbound request when the
// API sits behind the Cloudflare proxy (orange-cloud, tc-j222).
//
// With Cloudflare in front, r.RemoteAddr is a Cloudflare edge IP, not the
// caller. Cloudflare forwards the real client IP in the CF-Connecting-IP header.
// That header is only trustworthy when the TCP peer is genuinely Cloudflare:
// any other peer can forge it. So FromRequest trusts CF-Connecting-IP only when
// RemoteAddr falls inside a published Cloudflare IP range, and otherwise returns
// the peer address unchanged.
//
// X-Forwarded-For is deliberately never read: Cloudflare appends to it but it is
// trivially spoofable and offers nothing CF-Connecting-IP does not.
//
// # Deliberately unwired (privacy / UK GDPR)
//
// This package is the building block for FUTURE per-IP anonymous rate-limiting.
// It is intentionally NOT wired into logging, telemetry (no span client.address
// override), or the rate limiter (which stays keyed strictly on the
// authenticated subject, per tc-j222). Recording a client IP anywhere that
// persists or logs it is processing of personal data: it would require a
// Privacy Policy update (a legitimate-interest legal basis plus a stated
// retention period) and a Legitimate Interests Assessment FIRST, because the
// current policy discloses no IP logging and the data-minimisation principle
// (UK GDPR Art. 5(1)(c)) forbids collecting it ahead of a documented need. Until
// that decision is taken, callers may compute the IP for an in-memory,
// non-persisted purpose only; do not log, store, or export the returned value.
package clientip

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// cloudflareRanges is the vendored snapshot of Cloudflare's published edge IP
// ranges, parsed once at package initialisation into []netip.Prefix. There is no
// runtime fetch: the lists change rarely and a network dependency on startup
// would be a needless failure mode. Refresh from the source URLs below when
// Cloudflare publishes changes.
//
// Source: https://www.cloudflare.com/ips-v4 and https://www.cloudflare.com/ips-v6
// Snapshot fetched: 2026-06-19.
//
// MustParsePrefix is the idiomatic stdlib way to parse compile-time-constant
// prefixes; a parse failure here is a programmer error in this very list, caught
// the first time the package is loaded (including by tests), never in a live
// request.
var cloudflareRanges = []netip.Prefix{
	// IPv4 (https://www.cloudflare.com/ips-v4)
	netip.MustParsePrefix("173.245.48.0/20"),
	netip.MustParsePrefix("103.21.244.0/22"),
	netip.MustParsePrefix("103.22.200.0/22"),
	netip.MustParsePrefix("103.31.4.0/22"),
	netip.MustParsePrefix("141.101.64.0/18"),
	netip.MustParsePrefix("108.162.192.0/18"),
	netip.MustParsePrefix("190.93.240.0/20"),
	netip.MustParsePrefix("188.114.96.0/20"),
	netip.MustParsePrefix("197.234.240.0/22"),
	netip.MustParsePrefix("198.41.128.0/17"),
	netip.MustParsePrefix("162.158.0.0/15"),
	netip.MustParsePrefix("104.16.0.0/13"),
	netip.MustParsePrefix("104.24.0.0/14"),
	netip.MustParsePrefix("172.64.0.0/13"),
	netip.MustParsePrefix("131.0.72.0/22"),
	// IPv6 (https://www.cloudflare.com/ips-v6)
	netip.MustParsePrefix("2400:cb00::/32"),
	netip.MustParsePrefix("2606:4700::/32"),
	netip.MustParsePrefix("2803:f800::/32"),
	netip.MustParsePrefix("2405:b500::/32"),
	netip.MustParsePrefix("2405:8100::/32"),
	netip.MustParsePrefix("2a06:98c0::/29"),
	netip.MustParsePrefix("2c0f:f248::/32"),
}

// FromRequest resolves the real client IP for r.
//
// It parses the TCP peer from r.RemoteAddr. If that peer is within a Cloudflare
// range, it returns the IP from the CF-Connecting-IP header (falling back to the
// peer when the header is missing or unparseable). If the peer is not Cloudflare,
// it returns the peer and ignores any CF-Connecting-IP header (spoofable). The
// returned addr is the zero (invalid) Addr only when RemoteAddr cannot be parsed.
//
// See the package doc: the returned value must not be logged, persisted, or
// exported without a Privacy Policy update and an LIA.
func FromRequest(r *http.Request) netip.Addr {
	peer := peerAddr(r.RemoteAddr)
	if !peer.IsValid() || !isCloudflare(peer) {
		return peer
	}

	header := strings.TrimSpace(r.Header.Get("CF-Connecting-IP"))
	if header == "" {
		return peer
	}
	cfClient, err := netip.ParseAddr(header)
	if err != nil {
		return peer
	}
	return cfClient.Unmap()
}

// peerAddr parses RemoteAddr (host:port, with the host bracketed for IPv6) into a
// netip.Addr, normalising any IPv4-mapped IPv6 form so prefix-contains checks
// behave. It tolerates a bare host with no port. An unparseable value yields the
// zero Addr.
func peerAddr(remoteAddr string) netip.Addr {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// No port (or otherwise unsplittable): treat the whole string as the host.
		host = remoteAddr
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return addr.Unmap()
}

// isCloudflare reports whether addr falls within any vendored Cloudflare range.
func isCloudflare(addr netip.Addr) bool {
	for _, p := range cloudflareRanges {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}
