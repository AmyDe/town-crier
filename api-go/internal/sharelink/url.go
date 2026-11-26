package sharelink

import (
	"fmt"
	"strings"
)

const shareOrigin = "https://share.towncrierapp.uk"

// ApplicationURL returns the share-page URL for an application, or "" when
// either part is empty. The reference keeps its slashes (PlanIt references
// contain them) and each part between them is escaped with EscapeComponent.
func ApplicationURL(authoritySlug, ref string) string {
	if authoritySlug == "" || ref == "" {
		return ""
	}
	parts := strings.Split(ref, "/")
	for i, p := range parts {
		parts[i] = EscapeComponent(p)
	}
	return shareOrigin + "/a/" + authoritySlug + "/" + strings.Join(parts, "/")
}

// EscapeComponent mirrors JavaScript's encodeURIComponent: every byte outside
// A-Z a-z 0-9 - _ . ! ~ * ' ( ) is percent-encoded.
func EscapeComponent(s string) string {
	const unreserved = "-_.!~*'()"
	var b strings.Builder
	for i := range len(s) {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', strings.IndexByte(unreserved, c) >= 0:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
