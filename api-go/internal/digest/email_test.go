package digest

import (
	"strings"
	"testing"
	"time"

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
		"Home",                  // zone section header
		"Saved Applications",    // saved section header
		"10 High St",            // zone card address
		"5 Mill Ln",             // saved card address
		"Householder",           // zone card type
		"Rear extension",        // zone card description
		"2 new applications",    // footer count (plural)
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
