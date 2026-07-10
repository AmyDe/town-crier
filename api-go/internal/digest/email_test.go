package digest

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/designtokens"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
)

func strptr(s string) *string { return &s }

func testNotification(uid, zoneID, address, appType, desc string) notifications.DigestNotification {
	n := notifications.DigestNotification{
		ID:                     "n-" + uid,
		UserID:                 "user-1",
		ApplicationUID:         uid,
		ApplicationName:        uid,
		ApplicationAddress:     address,
		ApplicationDescription: desc,
		EventType:              notifications.EventNewApplication,
		AuthorityID:            1,
		CreatedAt:              time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	}
	if zoneID != "" {
		n.WatchZoneID = strptr(zoneID)
	}
	if appType != "" {
		n.ApplicationType = strptr(appType)
	}
	return n
}

func TestBuildDigestSubject(t *testing.T) {
	t.Parallel()
	got := buildDigestSubject(3)
	want := "Planning update — 3 new applications near you"
	if got != want {
		t.Errorf("subject: got %q, want %q", got, want)
	}
}

func TestBuildDigestHTML_RendersZoneAndSavedSections(t *testing.T) {
	t.Parallel()
	zones := []watchZoneDigest{
		{name: "Home", notifications: []notifications.DigestNotification{
			testNotification("19/00123/FUL", "zone-1", "10 High St", "Householder", "Rear extension"),
		}},
	}
	saved := []notifications.DigestNotification{
		testNotification("20/00045/FUL", "", "5 Mill Ln", "Full", "New dwelling"),
	}

	html := buildDigestHTML(zones, saved, 2)

	for _, want := range []string{
		"Town Crier",
		"Home",               // zone section header
		"Saved Applications", // saved section header
		"10 High St",         // zone card address
		"5 Mill Ln",          // saved card address
		"Householder",        // zone card type
		"Rear extension",     // zone card description
		"2 new applications", // footer count (plural)
		"towncrierapp.uk/applications/19/00123/FUL", // slash-preserving detail URL
	} {
		if !strings.Contains(html, want) {
			t.Errorf("digest HTML missing %q\n%s", want, html)
		}
	}
}

func TestBuildDigestHTML_DecisionLabelBadge(t *testing.T) {
	t.Parallel()
	// A decision-update notification renders the UK display label badge, not the
	// raw PlanIt app_state.
	n := testNotification("19/0009", "zone-1", "9 Oak Ave", "Householder", "Loft")
	n.EventType = notifications.EventDecisionUpdate
	n.Decision = strptr("Permitted")
	zones := []watchZoneDigest{{name: "Home", notifications: []notifications.DigestNotification{n}}}

	html := buildDigestHTML(zones, nil, 1)

	if !strings.Contains(html, "[Approved]") {
		t.Errorf("decision badge should show UK label [Approved], got:\n%s", html)
	}
	if strings.Contains(html, "[Permitted]") {
		t.Errorf("decision badge should not show raw PlanIt state Permitted")
	}
	// Singular footer copy.
	if !strings.Contains(html, "1 new application ") {
		t.Errorf("footer should use singular for count 1, got:\n%s", html)
	}
}

func TestBuildDigestHTML_SavedIndicatorOnZoneCard(t *testing.T) {
	t.Parallel()
	// A zone notification that also has the Saved source renders the "★ saved"
	// indicator on its zone card.
	n := testNotification("19/0010", "zone-1", "1 Elm Rd", "Householder", "Garage")
	n.Sources = "Zone, Saved"
	zones := []watchZoneDigest{{name: "Home", notifications: []notifications.DigestNotification{n}}}

	html := buildDigestHTML(zones, nil, 1)

	if !strings.Contains(html, "★ saved") {
		t.Errorf("expected saved indicator on zone card, got:\n%s", html)
	}
}

func TestBuildDigestHTML_EscapesUserContent(t *testing.T) {
	t.Parallel()
	// Application fields are HTML-encoded to prevent injection into the email body.
	n := testNotification("19/0011", "zone-1", "<script>alert(1)</script>", "Householder", "x & y")
	zones := []watchZoneDigest{{name: "Home & Garden", notifications: []notifications.DigestNotification{n}}}

	html := buildDigestHTML(zones, nil, 1)

	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Errorf("address must be HTML-encoded, got:\n%s", html)
	}
	if !strings.Contains(html, "Home &amp; Garden") {
		t.Errorf("zone name must be HTML-encoded, got:\n%s", html)
	}
}

func TestBuildApplicationDetailURL_PreservesSlashesEncodesSegments(t *testing.T) {
	t.Parallel()
	// Slash-containing PlanIt uids keep their slashes as path separators while
	// other reserved characters in a segment are percent-encoded.
	got := buildApplicationDetailURL("19/00123/FUL")
	want := "https://towncrierapp.uk/applications/19/00123/FUL"
	if got != want {
		t.Errorf("detail URL: got %q, want %q", got, want)
	}

	gotSpace := buildApplicationDetailURL("19/00 1")
	if !strings.Contains(gotSpace, "/applications/19/00%201") {
		t.Errorf("space in segment must be percent-encoded, got %q", gotSpace)
	}
}

// --- Public Notice restyle (tc-dn6hb / #859) ---

func TestBuildDigestHTML_NoLegacyBrandHexLiterals(t *testing.T) {
	t.Parallel()
	// The pre-brand navy/blue/grey literals must be fully gone: every colour
	// now comes from designtokens.
	n := testNotification("19/00123/FUL", "zone-1", "10 High St", "Householder", "Rear extension")
	zones := []watchZoneDigest{{name: "Home", notifications: []notifications.DigestNotification{n}}}
	html := buildDigestHTML(zones, nil, 1)

	for _, legacy := range []string{"#1a1a2e", "#4a6cf7", "#f0f0f0", "#eef1ff", "#fff3cd", "#f8f9fa", "#666", "#999", "#888", "#eee"} {
		if strings.Contains(html, legacy) {
			t.Errorf("legacy brand-colour literal %q should be gone, got:\n%s", legacy, html)
		}
	}
}

func TestBuildDigestHTML_UsesDesignTokenColours(t *testing.T) {
	t.Parallel()
	n := testNotification("19/00123/FUL", "zone-1", "10 High St", "Householder", "Rear extension")
	zones := []watchZoneDigest{{name: "Home", notifications: []notifications.DigestNotification{n}}}
	html := buildDigestHTML(zones, nil, 1)

	for _, want := range []string{
		designtokens.BackgroundLightHex,
		designtokens.SurfaceLightHex,
		designtokens.BorderLightHex,
		designtokens.TextPrimaryLightHex,
		designtokens.TextSecondaryLightHex,
		designtokens.AmberLightHex,
		designtokens.TextOnAccentLightHex,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("expected design token colour %q in digest HTML, got:\n%s", want, html)
		}
	}
}

func TestBuildDigestHTML_MastheadAndDoubleRule(t *testing.T) {
	t.Parallel()
	html := buildDigestHTML(nil, nil, 0)

	if !strings.Contains(html, "Town Crier") {
		t.Errorf("masthead should render the Town Crier wordmark, got:\n%s", html)
	}
	for _, testid := range []string{`data-testid="digest-masthead-rule-heavy"`, `data-testid="digest-masthead-rule-hairline"`} {
		if !strings.Contains(html, testid) {
			t.Errorf("masthead double rule missing %q, got:\n%s", testid, html)
		}
	}
}

func TestBuildDigestHTML_CTAIsVerbFirstAmber(t *testing.T) {
	t.Parallel()
	html := buildDigestHTML(nil, nil, 0)

	if !strings.Contains(html, "Open Town Crier") {
		t.Errorf("CTA label should be verb-first (\"Open Town Crier\"), got:\n%s", html)
	}
	if strings.Contains(html, "View All in App") {
		t.Errorf("old non-verb-first CTA label should be gone, got:\n%s", html)
	}
	if !strings.Contains(html, "border-radius:6px") {
		t.Errorf("CTA should have a 6px border radius, got:\n%s", html)
	}
}

func TestBuildDigestHTML_HeadlinesUseSansStack(t *testing.T) {
	t.Parallel()

	if headlineFontStack != bodyFontStack {
		t.Errorf("headlineFontStack should equal bodyFontStack, got headlineFontStack=%q bodyFontStack=%q", headlineFontStack, bodyFontStack)
	}

	n := testNotification("19/00123/FUL", "zone-1", "10 High St", "Householder", "Rear extension")
	zones := []watchZoneDigest{{name: "Home", notifications: []notifications.DigestNotification{n}}}
	html := buildDigestHTML(zones, nil, 1)

	if !strings.Contains(html, fmt.Sprintf(`font-family:%s;font-weight:700`, bodyFontStack)) {
		t.Errorf("headlines should render with the sans stack, got:\n%s", html)
	}
}

func TestBuildDigestHTML_ReferenceAndDateUseMonospace(t *testing.T) {
	t.Parallel()
	n := testNotification("19/00123/FUL", "zone-1", "10 High St", "Householder", "Rear extension")
	zones := []watchZoneDigest{{name: "Home", notifications: []notifications.DigestNotification{n}}}
	html := buildDigestHTML(zones, nil, 1)

	if !strings.Contains(html, "'Courier New', monospace") {
		t.Errorf("references/dates should use the Courier New monospace stack, got:\n%s", html)
	}
	if !strings.Contains(html, `data-testid="digest-notification-reference"`) {
		t.Errorf("expected a mono reference element, got:\n%s", html)
	}
	if !strings.Contains(html, "1 Feb 2026") {
		t.Errorf("expected the notification's created date rendered, got:\n%s", html)
	}
}

func TestBuildDigestHTML_ChipsAreOutlinedTransparent(t *testing.T) {
	t.Parallel()
	n := testNotification("19/0009", "zone-1", "9 Oak Ave", "Householder", "Loft")
	n.EventType = notifications.EventDecisionUpdate
	n.Decision = strptr("Permitted")
	n.Sources = "Zone, Saved"
	zones := []watchZoneDigest{{name: "Home", notifications: []notifications.DigestNotification{n}}}
	html := buildDigestHTML(zones, nil, 1)

	if !strings.Contains(html, "background:transparent") {
		t.Errorf("type chip and saved indicator should have a transparent background, got:\n%s", html)
	}
	if !strings.Contains(html, "border:1px solid "+designtokens.StatusPermittedLightHex) {
		t.Errorf("Approved decision chip should be outlined in the permitted status colour, got:\n%s", html)
	}
	if !strings.Contains(html, "border:1px solid "+designtokens.AmberLightHex) {
		t.Errorf("saved indicator should be outlined in amber, got:\n%s", html)
	}
}

func TestBuildDigestHTML_LightOnlyMetaTags(t *testing.T) {
	t.Parallel()
	html := buildDigestHTML(nil, nil, 0)

	if !strings.Contains(html, `<meta name="color-scheme" content="light">`) {
		t.Errorf("expected a light-only color-scheme meta tag, got:\n%s", html)
	}
	if !strings.Contains(html, `<meta name="supported-color-schemes" content="only light">`) {
		t.Errorf("expected an only-light supported-color-schemes hint, got:\n%s", html)
	}
}

func TestBuildDigestHTML_StructuralSafety(t *testing.T) {
	t.Parallel()
	n := testNotification("19/00123/FUL", "zone-1", "10 High St", "Householder", "Rear extension")
	zones := []watchZoneDigest{{name: "Home", notifications: []notifications.DigestNotification{n}}}
	html := buildDigestHTML(zones, nil, 1)

	if got := strings.Count(html, `width="600"`); got != 1 {
		t.Errorf("expected exactly one 600-wide table, got %d in:\n%s", got, html)
	}
	for _, forbidden := range []string{"display:flex", "display: flex", "display:grid", "display: grid", "grid-template", "<img", "@font-face", "fonts.googleapis"} {
		if strings.Contains(html, forbidden) {
			t.Errorf("email HTML must not contain %q, got:\n%s", forbidden, html)
		}
	}
	if !strings.Contains(html, "Unsubscribe") || !strings.Contains(html, "towncrierapp.uk/settings") {
		t.Errorf("expected an unsubscribe link, got:\n%s", html)
	}
}
