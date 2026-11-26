// Package clientip resolves the real client IP for an inbound request when the
// API sits behind the Cloudflare proxy (orange-cloud) and the Azure Container
// Apps (ACA) ingress.
//
// Cloudflare forwards the real client IP in the CF-Connecting-IP header, which is
// only trustworthy when the TCP peer is a proxy we trust: a Cloudflare edge range
// or the ACA platform range. FromRequest honours the header only for those peers
// and otherwise returns the peer address unchanged.
//
// X-Forwarded-For is deliberately never read: it is trivially spoofable and
// offers nothing CF-Connecting-IP does not.
//
// # Privacy (UK GDPR)
//
// The resolved IP feeds the in-memory anonymous rate limiter only. Never log,
// store, or export it (including span attributes): that would be processing of
// personal data needing a Privacy Policy update and an LIA first (UK GDPR
// Art. 5(1)(c)).
package clientip

import (
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strings"
)

// acaIngressRange is the ACA platform range the Envoy ingress connects from.
// Trusting it is safe because the ingress admits only Cloudflare IPv4 ranges, so
// only Cloudflare can set CF-Connecting-IP on a request that reaches us.
var acaIngressRange = netip.MustParsePrefix("100.100.0.0/16")

// cloudflareRanges is the vendored snapshot of Cloudflare's published edge IP
// ranges, parsed once at package initialisation into []netip.Prefix. There is no
// runtime fetch: the lists change rarely and a network dependency on startup
// would be a needless failure mode. Refresh from the source URLs below when
// Cloudflare publishes changes.
//
// Source: https://www.cloudflare.com/ips-v4 and https://www.cloudflare.com/ips-v6
// Snapshot fetched: 2026-06-19.
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
// range or the ACA ingress range, it returns the IP from the CF-Connecting-IP header (falling back to the
// peer when the header is missing or unparseable). If the peer is not a trusted proxy,
// it returns the peer and ignores any CF-Connecting-IP header (spoofable). The
// returned addr is the zero (invalid) Addr only when RemoteAddr cannot be parsed.
//
// See the package doc: the returned value must not be logged, persisted, or
// exported.
func FromRequest(r *http.Request) netip.Addr {
	peer := peerAddr(r.RemoteAddr)
	if !peer.IsValid() || !isTrustedProxy(peer) {
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
		host = remoteAddr
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return addr.Unmap()
}

func isTrustedProxy(addr netip.Addr) bool {
	return acaIngressRange.Contains(addr) || slices.ContainsFunc(cloudflareRanges, func(p netip.Prefix) bool { return p.Contains(addr) })
}
