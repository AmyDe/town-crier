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

// TestAddressIncludesPostcode pins the case/whitespace-insensitive suffix check
// behind the postcode-duplication fix (tc-r4n9.6): PlanIt addresses usually
// already end with the postcode, so appending it again in the h1 would render
// "... CR2 7DY, CR2 7DY".
func TestAddressIncludesPostcode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		address  string
		postcode string
		want     bool
	}{
		{
			name:     "address already ends with postcode",
			address:  "2 High Street, Croydon, CR2 7DY",
			postcode: "CR2 7DY",
			want:     true,
		},
		{
			name:     "address does not contain postcode",
			address:  "2 High Street, Croydon",
			postcode: "CR2 7DY",
			want:     false,
		},
		{
			name:     "case-insensitive match",
			address:  "2 High Street, Croydon, cr2 7dy",
			postcode: "CR2 7DY",
			want:     true,
		},
		{
			name:     "trailing whitespace on address is ignored",
			address:  "2 High Street, Croydon, CR2 7DY   ",
			postcode: "CR2 7DY",
			want:     true,
		},
		{
			name:     "trailing whitespace on postcode is ignored",
			address:  "2 High Street, Croydon, CR2 7DY",
			postcode: "  CR2 7DY  ",
			want:     true,
		},
		{
			name:     "empty postcode never matches",
			address:  "2 High Street, Croydon",
			postcode: "",
			want:     false,
		},
		{
			name:     "empty address never matches",
			address:  "",
			postcode: "CR2 7DY",
			want:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := addressIncludesPostcode(tt.address, tt.postcode); got != tt.want {
				t.Errorf("addressIncludesPostcode(%q, %q) = %v, want %v", tt.address, tt.postcode, got, tt.want)
			}
		})
	}
}

// TestBuildPageView_PostcodeDeduplication pins buildPageView's use of
// addressIncludesPostcode: the view's Postcode field (the h1's ", {postcode}"
// suffix) is suppressed when the address already carries it, and populated
// unchanged otherwise.
func TestBuildPageView_PostcodeDeduplication(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		address      string
		postcode     *string
		wantPostcode string
	}{
		{
			name:         "address already includes postcode: suffix suppressed",
			address:      "2 High Street, Croydon, CR2 7DY",
			postcode:     ptr("CR2 7DY"),
			wantPostcode: "",
		},
		{
			name:         "address does not include postcode: unchanged",
			address:      "2 High Street, Croydon",
			postcode:     ptr("CR2 7DY"),
			wantPostcode: "CR2 7DY",
		},
		{
			name:         "nil postcode: unchanged (empty)",
			address:      "2 High Street, Croydon",
			postcode:     nil,
			wantPostcode: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := applications.PlanningApplication{
				Name:     "23/03456/FUL",
				AreaID:   165,
				Address:  tt.address,
				Postcode: tt.postcode,
			}
			v := buildPageView(app, "croydon", "23/03456/FUL")
			if v.Postcode != tt.wantPostcode {
				t.Errorf("Postcode = %q, want %q", v.Postcode, tt.wantPostcode)
			}
		})
	}
}

// TestStatusChip pins the shared status vocabulary/palette (tc-r4n9 decision
// 4): the resident-facing label mirrors web/scripts/lib/format.mjs's
// STATUS_DISPLAY_LABEL_MAP, and the colour modifier deliberately collapses to
// three buckets — granted (green), refused (red), neutral for everything else
// including "Undecided" and every long-tail state — rather than a five-way
// traffic light.
func TestStatusChip(t *testing.T) {
	t.Parallel()
	tests := []struct {
		appState     string
		wantLabel    string
		wantModifier string
	}{
		{appState: "Permitted", wantLabel: "Granted", wantModifier: "granted"},
		{appState: "Conditions", wantLabel: "Granted with conditions", wantModifier: "granted"},
		{appState: "Rejected", wantLabel: "Refused", wantModifier: "refused"},
		{appState: "Undecided", wantLabel: "Undecided", wantModifier: "neutral"},
		{appState: "Withdrawn", wantLabel: "Withdrawn", wantModifier: "neutral"},
		{appState: "Appealed", wantLabel: "Appealed", wantModifier: "neutral"},
		{appState: "Unresolved", wantLabel: "Unresolved", wantModifier: "neutral"},
		{appState: "Referred", wantLabel: "Referred", wantModifier: "neutral"},
		{appState: "Some Future PlanIt State", wantLabel: "Some Future PlanIt State", wantModifier: "neutral"},
	}
	for _, tt := range tests {
		t.Run(tt.appState, func(t *testing.T) {
			t.Parallel()
			gotLabel, gotModifier := statusChip(tt.appState)
			if gotLabel != tt.wantLabel || gotModifier != tt.wantModifier {
				t.Errorf("statusChip(%q) = (%q, %q), want (%q, %q)", tt.appState, gotLabel, gotModifier, tt.wantLabel, tt.wantModifier)
			}
		})
	}
}

// TestBuildPageView_StatusChip pins buildPageView's wiring of the mapped
// vocabulary: a nil AppState leaves both fields empty (so the template omits
// the chip entirely); a set AppState is run through statusChip.
func TestBuildPageView_StatusChip(t *testing.T) {
	t.Parallel()
	t.Run("nil AppState omits the chip", func(t *testing.T) {
		t.Parallel()
		app := applications.PlanningApplication{Name: "23/03456/FUL", AreaID: 165}
		v := buildPageView(app, "croydon", "23/03456/FUL")
		if v.StatusLabel != "" || v.StatusModifier != "" {
			t.Errorf("StatusLabel/StatusModifier = %q/%q, want empty/empty", v.StatusLabel, v.StatusModifier)
		}
	})
	t.Run("set AppState is mapped, not passed through raw", func(t *testing.T) {
		t.Parallel()
		app := applications.PlanningApplication{Name: "23/03456/FUL", AreaID: 165, AppState: ptr("Permitted")}
		v := buildPageView(app, "croydon", "23/03456/FUL")
		if v.StatusLabel != "Granted" || v.StatusModifier != "granted" {
			t.Errorf("StatusLabel/StatusModifier = %q/%q, want Granted/granted", v.StatusLabel, v.StatusModifier)
		}
	})
}
