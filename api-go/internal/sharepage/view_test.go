package sharepage

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
)

// TestBuildPageView_OGImagePointsAtGeneratedCard pins the Slice-2 repoint: the
// unfurl image is the generated OSM map-card route for this exact (slug, ref),
// built from the share origin — not a static placeholder. The route itself
// decides map vs branded fallback, so the page always points here.
func TestBuildPageView_OGImagePointsAtGeneratedCard(t *testing.T) {
	t.Parallel()
	v := buildPageView(applications.PlanningApplication{Name: "23/03456/FUL", AreaID: 165}, "croydon", "23/03456/FUL")
	want := "https://share.towncrierapp.uk/og/croydon/23/03456/FUL.png"
	if v.OGImage != want {
		t.Errorf("OGImage = %q, want %q", v.OGImage, want)
	}
}

// TestBuildPageView_HomeURLPointsAtTownCrierHomepage pins Problem 3 (#763,
// tc-iuf0): the share page has no per-application web destination, so the
// homepage link always points at the Town Crier marketing site, regardless of
// device or the (slug, ref) being viewed.
func TestBuildPageView_HomeURLPointsAtTownCrierHomepage(t *testing.T) {
	t.Parallel()
	v := buildPageView(applications.PlanningApplication{Name: "23/03456/FUL", AreaID: 165}, "croydon", "23/03456/FUL")
	want := "https://towncrierapp.uk"
	if v.HomeURL != want {
		t.Errorf("HomeURL = %q, want %q", v.HomeURL, want)
	}
}

// TestSummarise_TruncatesMultibyteRuneSafe pins the truncation branch on a
// proposal built entirely from 3-byte CJK runes and well over the cap. A byte-wise
// truncation would split a rune and yield U+FFFD; the rune-wise implementation
// cannot. Guards against a regression to byte slicing that would corrupt the
// og:/twitter: description on any non-ASCII proposal.
func TestSummarise_TruncatesMultibyteRuneSafe(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("界", 250) // 250 runes, 750 bytes, > the 200-rune cap
	got := summarise(long, "")

	if !utf8.ValidString(got) {
		t.Fatalf("result is not valid UTF-8: %q", got)
	}
	if strings.ContainsRune(got, '�') {
		t.Error("result contains the Unicode replacement char — a multibyte rune was split")
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("truncated summary must end with an ellipsis, got %q", got)
	}
	if n := utf8.RuneCountInString(got); n > ogDescriptionMaxRunes+1 {
		t.Errorf("summary rune length = %d, want <= %d (cap + ellipsis)", n, ogDescriptionMaxRunes+1)
	}
	if want := strings.Repeat("界", ogDescriptionMaxRunes) + "…"; got != want {
		t.Errorf("summary = %q, want the first %d runes + ellipsis", got, ogDescriptionMaxRunes)
	}
}

// TestSummarise_ShortMultibyteReturnedWhole covers the under-cap path with 2-byte
// accented runes: the proposal is returned verbatim, untruncated and rune-safe.
func TestSummarise_ShortMultibyteReturnedWhole(t *testing.T) {
	t.Parallel()
	in := "Réfection d'une façade au cœur du café"
	if got := summarise(in, "Café Royal"); got != in {
		t.Errorf("summary = %q, want the input returned verbatim", got)
	}
}

// TestSummarise_EmptyDescriptionWithPlace pins the place-based fallback sentence
// used when the record carries no proposal text but does have an address/area.
func TestSummarise_EmptyDescriptionWithPlace(t *testing.T) {
	t.Parallel()
	got := summarise("   ", "10 Downing Street, London")
	want := "Planning application at 10 Downing Street, London. View the details on Town Crier."
	if got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
}

// TestSummarise_EmptyDescriptionNoPlace pins the sane default when neither a
// proposal nor a place is available.
func TestSummarise_EmptyDescriptionNoPlace(t *testing.T) {
	t.Parallel()
	got := summarise("", "")
	want := "View this planning application on Town Crier."
	if got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
}
