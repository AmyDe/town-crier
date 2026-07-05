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

// TestAnonymousPatterns_IncludesApplicationSearch pins the anonymity of the
// public application search endpoint (#821 Phase 3, tc-geq7h.3): a resident
// searching by reference/address/description needs no token, mirroring the
// by-slug application read. The middleware keys on the exact registered
// pattern string, so this must match the route wired in applications
// byte-for-byte.
func TestAnonymousPatterns_IncludesApplicationSearch(t *testing.T) {
	t.Parallel()

	const search = "GET /v1/applications/search"
	if _, ok := anonymousPatterns[search]; !ok {
		t.Errorf("anonymousPatterns must include %q", search)
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

// TestAnonymousPatterns_IncludesAASA pins the anonymity of the Apple App Site
// Association document served on the share host (#738 Slice 3). Apple's daemon
// fetches it without a bearer token, so the exact extensionless well-known path
// must be present in the map; the auth middleware keys on the registered pattern
// string byte-for-byte.
func TestAnonymousPatterns_IncludesAASA(t *testing.T) {
	t.Parallel()

	const aasa = "GET /.well-known/apple-app-site-association"
	if _, ok := anonymousPatterns[aasa]; !ok {
		t.Errorf("anonymousPatterns must include %q", aasa)
	}
}

// TestAnonymousPatterns_IncludesAssetLinks pins the anonymity of the Android
// Digital Asset Links document served on the share host (GH#782). Android's
// package manager fetches it without a bearer token to verify the app's
// autoVerify intent filter, mirroring the AASA entry above; the auth
// middleware keys on the registered pattern string byte-for-byte.
func TestAnonymousPatterns_IncludesAssetLinks(t *testing.T) {
	t.Parallel()

	const assetLinks = "GET /.well-known/assetlinks.json"
	if _, ok := anonymousPatterns[assetLinks]; !ok {
		t.Errorf("anonymousPatterns must include %q", assetLinks)
	}
}
