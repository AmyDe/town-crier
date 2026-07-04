package sharepage

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
)

// fakeStore is a hand-written double for the application point read. It records
// how it was called so tests can assert the (authorityCode, ref) it received.
type fakeStore struct {
	app              applications.PlanningApplication
	found            bool
	err              error
	gotAuthorityCode string
	gotName          string
	calls            int
}

func (f *fakeStore) GetByAuthorityAndName(_ context.Context, authorityCode, name string) (applications.PlanningApplication, bool, error) {
	f.calls++
	f.gotAuthorityCode = authorityCode
	f.gotName = name
	if f.err != nil {
		return applications.PlanningApplication{}, false, f.err
	}
	return f.app, f.found, nil
}

// fakeResolver is a hand-written double for the authority slug resolver.
type fakeResolver struct {
	slugs map[string]int
}

func (f *fakeResolver) SlugToAreaID(slug string) (int, bool) {
	id, ok := f.slugs[slug]
	return id, ok
}

func ptr[T any](v T) *T { return &v }

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("parse date %q: %v", s, err)
	}
	return d
}

// fullApp is a fully-populated snapshot: every optional field set, so a single
// render exercises the chip, the timeline and both official-record links.
func fullApp(t *testing.T) applications.PlanningApplication {
	t.Helper()
	return applications.PlanningApplication{
		Name:          "23/03456/FUL",
		UID:           "croydon-23-03456-FUL",
		AreaName:      "Croydon",
		AreaID:        165,
		Address:       "10 Downing Street, London",
		Postcode:      ptr("CR0 1AB"),
		Description:   "Erection of a two-storey rear extension.",
		AppType:       ptr("Full planning permission"),
		AppState:      ptr("Under Consideration"),
		StartDate:     ptr(mustDate(t, "2024-03-02")),
		ConsultedDate: ptr(mustDate(t, "2024-03-10")),
		DecidedDate:   ptr(mustDate(t, "2024-04-15")),
		URL:           ptr("https://www.croydon.gov.uk/planning/123"),
		Link:          ptr("https://planit.org.uk/planapplic/165/23/03456/FUL"),
	}
}

// serve wires Routes onto a real mux and drives the given path through it, so the
// "GET /a/{authoritySlug}/{ref...}" pattern (and its slash-carrying ref capture)
// is exercised, not just the handler in isolation. The request carries no bearer
// token — the page is anonymous.
func serve(t *testing.T, store appStore, resolver slugResolver, path string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	Routes(mux, store, resolver, slog.New(slog.DiscardHandler))
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	mux.ServeHTTP(rec, req)
	return rec
}

func TestServe_KnownApplication_RendersMetaAndVisibleContent(t *testing.T) {
	t.Parallel()
	store := &fakeStore{app: fullApp(t), found: true}
	resolver := &fakeResolver{slugs: map[string]int{"croydon": 165}}

	rec := serve(t, store, resolver, "/a/croydon/23/03456/FUL")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html; charset=utf-8", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=3600" {
		t.Errorf("Cache-Control = %q, want public, max-age=3600", got)
	}
	// Anonymity: the page must set no cookies.
	if cookies := rec.Header().Values("Set-Cookie"); len(cookies) != 0 {
		t.Errorf("share page set cookies %v; must be anonymous", cookies)
	}
	// The store is point-read by the resolved area id (stringified) and the raw ref.
	if store.gotAuthorityCode != "165" || store.gotName != "23/03456/FUL" {
		t.Errorf("store queried with (%q,%q), want (165, 23/03456/FUL)", store.gotAuthorityCode, store.gotName)
	}

	body := rec.Body.String()
	wantContains := []string{
		// HEAD / meta asserted by the acceptance criteria.
		`<meta name="robots" content="noindex,follow">`,
		`<link rel="canonical" href="https://share.towncrierapp.uk/a/croydon/23/03456/FUL">`,
		// og:/twitter: titles and descriptions asserted by CONTENT, not mere
		// presence: og:title carries the ref+place headline; og:description carries
		// the proposal summary. A blank or wrong unfurl would slip past an
		// attribute-presence check but fails these.
		`property="og:title" content="23/03456/FUL · 10 Downing Street, London"`,
		`property="og:description" content="Erection of a two-storey rear extension."`,
		`property="og:url" content="https://share.towncrierapp.uk/a/croydon/23/03456/FUL"`,
		`property="og:site_name" content="Town Crier"`,
		`property="og:type"`,
		`name="twitter:card" content="summary_large_image"`,
		`name="twitter:title" content="23/03456/FUL · 10 Downing Street, London"`,
		`name="twitter:description" content="Erection of a two-storey rear extension."`,
		`name="twitter:image"`,
		`name="apple-itunes-app" content="app-id=6764095657`,
		// og:image AND twitter:image both point at the generated OSM map-card route
		// (the branded fallback is now baked by that route, not a static URL).
		`property="og:image" content="https://share.towncrierapp.uk/og/croydon/23/03456/FUL.png"`,
		`name="twitter:image" content="https://share.towncrierapp.uk/og/croydon/23/03456/FUL.png"`,
		// Visible essentials.
		"10 Downing Street, London",
		"CR0 1AB",
		"23/03456/FUL",
		"Full planning permission", // AppType, rendered in the reference row
		"Erection of a two-storey rear extension.",
		"Under Consideration",
		// Key-dates timeline (doubles as status history): the heading and every
		// row's LABEL and human-formatted VALUE. fullApp sets all three dates, so a
		// missing timeline or a mis-formatted date is caught here.
		"<h2>Key dates</h2>",
		`<span class="date-label">Started</span><span class="date-value">2 March 2024</span>`,
		`<span class="date-label">Consulted</span><span class="date-value">10 March 2024</span>`,
		`<span class="date-label">Decided</span><span class="date-value">15 April 2024</span>`,
		// Official-record links (PlanIt primary, council secondary): nofollow +
		// noopener + noreferrer (noreferrer strips the Referer to third-party sites).
		`rel="nofollow noopener noreferrer"`,
		"https://planit.org.uk/planapplic/165/23/03456/FUL",
		"https://www.croydon.gov.uk/planning/123",
		// Attribution footer — the four ADR-0006 lines, verbatim.
		"Planning data provided by PlanIt (planit.org.uk)",
		"Contains public sector information licensed under the Open Government Licence. Crown Copyright.",
		"Contains Ordnance Survey data © Crown Copyright and database right.",
		"Map data © OpenStreetMap contributors.",
		// Sticky CTA: exact copy + the campaign-tagged App Store URL.
		"Download the app for instant notifications about planning updates",
		"town-crier-planning-alerts",
		"ct=share-page",
		"mt=8",
	}
	for _, want := range wantContains {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

// TestServe_RendersMapHeroInBody pins Problem 2 (#763, tc-9ui9): the baked OSM
// map card was previously emitted ONLY as og:image/twitter:image in <head>, so
// browsers never showed it — only social unfurls did. The page body must carry
// a visible hero <img> pointing at the SAME cached card URL as the unfurl meta.
func TestServe_RendersMapHeroInBody(t *testing.T) {
	t.Parallel()
	store := &fakeStore{app: fullApp(t), found: true}
	resolver := &fakeResolver{slugs: map[string]int{"croydon": 165}}

	rec := serve(t, store, resolver, "/a/croydon/23/03456/FUL")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<img class="hero"`) {
		t.Error("body missing a hero <img> — the map must render in the page body, not just og:image")
	}
	wantSrc := `src="https://share.towncrierapp.uk/og/croydon/23/03456/FUL.png"`
	if !strings.Contains(body, wantSrc) {
		t.Errorf("hero <img> src missing or wrong; body should contain %q", wantSrc)
	}
}

// TestServe_RendersHomepageLink pins Problem 3 (#763, tc-iuf0): the only CTA was
// the iOS-only App Store bar, with no link to the web app anywhere. A visible,
// always-present link to the Town Crier homepage must be present on every device.
func TestServe_RendersHomepageLink(t *testing.T) {
	t.Parallel()
	store := &fakeStore{app: fullApp(t), found: true}
	resolver := &fakeResolver{slugs: map[string]int{"croydon": 165}}

	rec := serve(t, store, resolver, "/a/croydon/23/03456/FUL")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `href="https://towncrierapp.uk"`) {
		t.Error("body missing a link to the Town Crier homepage (https://towncrierapp.uk)")
	}
}

// TestServe_RendersHeaderAboveHeroWithBrandLockup pins Phase 5 (#794): the
// floating "• Town Crier" legend-dot below the map is replaced by a real
// header element above the hero, whose brand lockup links to the marketing
// homepage.
func TestServe_RendersHeaderAboveHeroWithBrandLockup(t *testing.T) {
	t.Parallel()
	store := &fakeStore{app: fullApp(t), found: true}
	resolver := &fakeResolver{slugs: map[string]int{"croydon": 165}}

	rec := serve(t, store, resolver, "/a/croydon/23/03456/FUL")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()

	headerIdx := strings.Index(body, `<header class="site-header">`)
	if headerIdx == -1 {
		t.Fatal(`body missing <header class="site-header">`)
	}
	brandWant := `<a class="brand-link" href="https://towncrierapp.uk">Town Crier</a>`
	if !strings.Contains(body, brandWant) {
		t.Errorf("body missing brand lockup %q", brandWant)
	}
	heroIdx := strings.Index(body, `<img class="hero"`)
	if heroIdx == -1 {
		t.Fatal("body missing hero image")
	}
	if headerIdx > heroIdx {
		t.Error("header must render above the hero image, not below it")
	}
}

// TestServe_AppCTAIsPrimaryInCardAction pins decision 2 (#794): the App Store
// CTA is the visually primary action inside the card, rendered above the
// "Official record" section — whose PlanIt/council links are restyled
// secondary/tertiary so an external link never outweighs our own CTA.
func TestServe_AppCTAIsPrimaryInCardAction(t *testing.T) {
	t.Parallel()
	store := &fakeStore{app: fullApp(t), found: true}
	resolver := &fakeResolver{slugs: map[string]int{"croydon": 165}}

	rec := serve(t, store, resolver, "/a/croydon/23/03456/FUL")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()

	ctaIdx := strings.Index(body, `<div class="app-cta">`)
	if ctaIdx == -1 {
		t.Fatal(`body missing <div class="app-cta">`)
	}
	officialIdx := strings.Index(body, ">Official record<")
	if officialIdx == -1 {
		t.Fatal("body missing Official record section")
	}
	if ctaIdx > officialIdx {
		t.Fatal("in-card app CTA must render above the Official record section")
	}

	appCTASection := body[ctaIdx:officialIdx]
	if !strings.Contains(appCTASection, `class="btn-primary"`) {
		t.Error(`in-card app CTA missing a "btn-primary" button`)
	}
	if !strings.Contains(appCTASection, ">Get the app<") {
		t.Error(`in-card app CTA button must use the short, verb-first label "Get the app"`)
	}
	if !strings.Contains(appCTASection, "apps.apple.com") {
		t.Error("in-card app CTA must link to the App Store")
	}

	if !strings.Contains(body, `class="record-link secondary"`) {
		t.Error(`PlanIt link must carry class "record-link secondary"`)
	}
	if !strings.Contains(body, `class="record-link tertiary"`) {
		t.Error(`council link must carry class "record-link tertiary"`)
	}
	if strings.Contains(body, `class="record-link primary"`) {
		t.Error(`no official-record link may carry the "primary" class — that is now reserved for the app CTA`)
	}
}

func TestServe_UnknownSlug_RendersBranded404(t *testing.T) {
	t.Parallel()
	store := &fakeStore{}
	resolver := &fakeResolver{slugs: map[string]int{"croydon": 165}}

	rec := serve(t, store, resolver, "/a/nowhere/23/0001/FUL")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if store.calls != 0 {
		t.Errorf("store queried %d times for an unknown slug, want 0", store.calls)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", got)
	}
	if rec.Body.Len() == 0 {
		t.Error("404 body is empty, want branded HTML")
	}
	if !strings.Contains(rec.Body.String(), "Town Crier") {
		t.Error("404 body is not branded (missing Town Crier)")
	}
	// A just-ingested application must not be negatively cached for an hour.
	if strings.Contains(rec.Header().Get("Cache-Control"), "max-age=3600") {
		t.Errorf("404 Cache-Control = %q must not carry max-age=3600", rec.Header().Get("Cache-Control"))
	}
}

func TestServe_UnknownRef_RendersBranded404(t *testing.T) {
	t.Parallel()
	store := &fakeStore{found: false}
	resolver := &fakeResolver{slugs: map[string]int{"croydon": 165}}

	rec := serve(t, store, resolver, "/a/croydon/99/9999/XXX")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", got)
	}
	if rec.Body.Len() == 0 {
		t.Error("404 body is empty, want branded HTML")
	}
	if strings.Contains(rec.Header().Get("Cache-Control"), "max-age=3600") {
		t.Errorf("404 Cache-Control = %q must not carry max-age=3600", rec.Header().Get("Cache-Control"))
	}
}

func TestServe_StoreError_Returns500(t *testing.T) {
	t.Parallel()
	store := &fakeStore{err: errors.New("db down")}
	resolver := &fakeResolver{slugs: map[string]int{"croydon": 165}}

	rec := serve(t, store, resolver, "/a/croydon/23/0001/FUL")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestServe_EscapesUntrustedFields(t *testing.T) {
	t.Parallel()
	app := fullApp(t)
	app.Description = "<script>alert(1)</script>"
	app.Address = `"><script>alert(2)</script>`
	store := &fakeStore{app: app, found: true}
	resolver := &fakeResolver{slugs: map[string]int{"croydon": 165}}

	rec := serve(t, store, resolver, "/a/croydon/23/03456/FUL")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("Description not escaped: raw <script> present in output")
	}
	if strings.Contains(body, "<script>alert(2)</script>") {
		t.Error("Address not escaped: raw <script> present in output")
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Error("expected HTML-escaped <script> in output (proves html/template contextual escaping)")
	}
}

// TestServe_NeutralisesJavascriptSchemeInOfficialLinks pins the URL-context
// escaper (distinct from the text-context escaper above): a hostile
// "javascript:" scheme in the PlanIt/council record URLs must NOT survive into a
// clickable href. html/template's URL filter rewrites an unsafe scheme to the
// sentinel "#ZgotmplZ". Asserting the sentinel (and the absence of the raw scheme)
// means a future move off html/template cannot silently reintroduce a clickable
// javascript: link.
func TestServe_NeutralisesJavascriptSchemeInOfficialLinks(t *testing.T) {
	t.Parallel()
	app := fullApp(t)
	app.URL = ptr("javascript:alert(1)")  // council secondary link
	app.Link = ptr("javascript:alert(2)") // PlanIt primary link
	store := &fakeStore{app: app, found: true}
	resolver := &fakeResolver{slugs: map[string]int{"croydon": 165}}

	rec := serve(t, store, resolver, "/a/croydon/23/03456/FUL")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "javascript:alert(1)") {
		t.Error("council href retained raw javascript: scheme — URL filter bypassed")
	}
	if strings.Contains(body, "javascript:alert(2)") {
		t.Error("PlanIt href retained raw javascript: scheme — URL filter bypassed")
	}
	if !strings.Contains(body, "#ZgotmplZ") {
		t.Error("expected html/template's #ZgotmplZ sentinel for the neutralised javascript: scheme")
	}
}

// TestServe_EscapesHostileRefInReflectionPoints guards the attribute/URL-context
// escaping of the attacker-controlled trailing ref, which is reflected into the
// canonical <link href>, the og:url content, the <title>, and the
// apple-itunes-app app-argument. A ref that closes the attribute and injects a
// <script> must not appear raw in ANY of those places. Rendered directly through
// the template so the hostile bytes are not mangled by request URL parsing first.
func TestServe_EscapesHostileRefInReflectionPoints(t *testing.T) {
	t.Parallel()
	const hostileRef = `23/03456/FUL"><script>alert(1)</script>`
	view := buildPageView(fullApp(t), "croydon", hostileRef)

	var buf bytes.Buffer
	if err := pageTemplates.ExecuteTemplate(&buf, "page", view); err != nil {
		t.Fatalf("render: %v", err)
	}
	body := buf.String()

	// The raw break-out sequence must not survive into the output at all.
	if strings.Contains(body, `"><script>`) {
		t.Error(`raw "><script> break-out sequence present — attribute/URL escaping failed`)
	}
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("raw <script> injected via ref present in output")
	}
	// Sanity: the benign portion of the ref is still rendered (escaping did not
	// simply drop everything), so the assertion above is meaningful.
	if !strings.Contains(body, "23/03456/FUL") {
		t.Error("benign ref prefix missing — render produced nothing to escape")
	}
}

func TestServe_NilOptionalFields_Renders200WithoutPanic(t *testing.T) {
	t.Parallel()
	app := applications.PlanningApplication{
		Name:     "24/0001/OUT",
		AreaName: "Croydon",
		AreaID:   165,
		Address:  "Land at Foo Road",
		// Postcode, AppType, AppState, all dates, URL and Link are nil;
		// Description is empty.
	}
	store := &fakeStore{app: app, found: true}
	resolver := &fakeResolver{slugs: map[string]int{"croydon": 165}}

	rec := serve(t, store, resolver, "/a/croydon/24/0001/OUT")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Land at Foo Road") {
		t.Error("address missing")
	}
	if !strings.Contains(body, "24/0001/OUT") {
		t.Error("ref missing")
	}
	// Omitted sections must be absent.
	if strings.Contains(body, `class="chip"`) {
		t.Error("status chip rendered despite nil AppState")
	}
	if strings.Contains(body, `rel="nofollow noopener noreferrer"`) {
		t.Error("official-record link rendered despite nil URL/Link")
	}
	if strings.Contains(body, ">Key dates<") {
		t.Error("key-dates section rendered despite no dates")
	}
	// Attribution + CTA still present.
	if !strings.Contains(body, "Download the app for instant notifications about planning updates") {
		t.Error("sticky CTA missing")
	}
	if !strings.Contains(body, "Map data © OpenStreetMap contributors.") {
		t.Error("attribution missing")
	}
}
