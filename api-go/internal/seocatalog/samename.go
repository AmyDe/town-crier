package seocatalog

import (
	"regexp"
	"strings"

	"github.com/AmyDe/town-crier/api-go/internal/authorities"
)

var (
	trailingParenthetical = regexp.MustCompile(`\s*\([^)]*\)\s*$`)
	administrativeSuffix  = regexp.MustCompile(`(?i),\s*(?:city|county)\s+of\s*$`)
)

// canonicalAuthorityName strips bilingual and administrative decorations
// ("Wrexham / Wrecsam", "Bristol, City of") so the name compares with a town
// slug.
func canonicalAuthorityName(name string) string {
	n := name
	if i := strings.Index(n, "/"); i != -1 {
		n = n[:i]
	}
	n = trailingParenthetical.ReplaceAllString(n, "")
	n = administrativeSuffix.ReplaceAllString(n, "")
	return strings.TrimSpace(n)
}

// isSameNameAsAuthority reports whether a town page would duplicate its
// authority page: the town slug equals the authority's slug, raw or normalised.
func isSameNameAsAuthority(authorityName, townSlug string) bool {
	if authorities.Slugify(authorityName) == townSlug {
		return true
	}
	return authorities.Slugify(canonicalAuthorityName(authorityName)) == townSlug
}
