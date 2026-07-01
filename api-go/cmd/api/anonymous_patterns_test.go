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
