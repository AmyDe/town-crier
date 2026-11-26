package webhooks

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"slices"
	"strings"
)

// ErrInvalidURL reports a webhook URL that must not be saved. The error text
// names the broken rule and never carries the URL.
var ErrInvalidURL = errors.New("webhooks: invalid url")

const (
	maxURLBytes = 2048
	httpsPort   = "443"
)

var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	// Carrier-grade NAT. Azure Container Apps ingress uses 100.100.x.x.
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	// 6to4 and IPv4-compatible addresses embed an IPv4 address, which can be
	// a private one. fec0::/10 is the deprecated site-local range.
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("::/96"),
	netip.MustParsePrefix("fec0::/10"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("2001:db8::/32"),
}

// isPublicAddr reports whether a webhook may connect to addr.
func isPublicAddr(addr netip.Addr) bool {
	// Prefix.Contains never matches a zoned address, so a zone must fail first.
	if addr.Zone() != "" {
		return false
	}
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() ||
		addr.IsLinkLocalUnicast() || addr.IsMulticast() || addr.IsUnspecified() {
		return false
	}
	return !slices.ContainsFunc(blockedPrefixes, func(p netip.Prefix) bool { return p.Contains(addr) })
}

// ValidateURL checks a webhook URL at save time. It accepts only an https URL
// of at most 2048 bytes with a host, no userinfo and no port other than 443,
// whose host is not a non-public IP literal or a localhost, .local or
// .internal name. It does no DNS lookup: the delivery client checks every
// dialled address. A rejected URL returns an error wrapping ErrInvalidURL.
func ValidateURL(raw string) error {
	if len(raw) > maxURLBytes {
		return invalidURL("longer than 2048 bytes")
	}
	u, err := url.Parse(raw)
	if err != nil {
		// The parse error quotes the URL, so it is not wrapped.
		return invalidURL("not parseable")
	}
	if u.Scheme != "https" {
		return invalidURL("scheme is not https")
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" {
		return invalidURL("no host")
	}
	if u.User != nil {
		return invalidURL("has userinfo")
	}
	if port := u.Port(); port != "" && port != httpsPort {
		return invalidURL("port is not 443")
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		if !isPublicAddr(addr) {
			return invalidURL("address is not public")
		}
		return nil
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") ||
		strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return invalidURL("host is a local name")
	}
	return nil
}

func invalidURL(rule string) error {
	return fmt.Errorf("%w: %s", ErrInvalidURL, rule)
}
