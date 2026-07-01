package main

import "testing"

// TestAnonymousPatterns_IncludesBySlugApplicationRead pins the anonymity of the
// public by-slug application read (#738). The middleware keys on the exact
// registered pattern string, so this must match the route wired in applications
// byte-for-byte. The sibling by-id read stays authed (absent from the map).
func TestAnonymousPatterns_IncludesBySlugApplicationRead(t *testing.T) {
	t.Parallel()

	const bySlug = "GET /v1/applications/by-slug/{authoritySlug}/{ref...}"
	if _, ok := anonymousPatterns[bySlug]; !ok {
		t.Errorf("anonymousPatterns must include %q", bySlug)
	}

	const byID = "GET /v1/applications/{authorityCode}/{name...}"
	if _, ok := anonymousPatterns[byID]; ok {
		t.Errorf("the by-id read must stay authed; %q must NOT be anonymous", byID)
	}
}

// TestAnonymousPatterns_IncludesSharePage pins the anonymity of the public
// server-rendered share page (#738). The auth middleware keys on the exact
// registered pattern string, so this must match the route wired in sharepage
// byte-for-byte.
func TestAnonymousPatterns_IncludesSharePage(t *testing.T) {
	t.Parallel()

	const sharePage = "GET /a/{authoritySlug}/{ref...}"
	if _, ok := anonymousPatterns[sharePage]; !ok {
		t.Errorf("anonymousPatterns must include %q", sharePage)
	}
}

// TestAnonymousPatterns_IncludesShareCardImage pins the anonymity of the public
// og:image map-card route (#738 Slice 2). The ".png" suffix is enforced inside
// the handler, not in the mux pattern (the {ref...} wildcard must be final), so
// the registered pattern is suffix-free and must match this byte-for-byte.
func TestAnonymousPatterns_IncludesShareCardImage(t *testing.T) {
	t.Parallel()

	const shareCard = "GET /og/{authoritySlug}/{ref...}"
	if _, ok := anonymousPatterns[shareCard]; !ok {
		t.Errorf("anonymousPatterns must include %q", shareCard)
	}
}
