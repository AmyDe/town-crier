package sharepage

import (
	"strings"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
)

const (
	// shareOrigin is the intended public origin for the share page. Canonical and
	// og:url point here even though this slice serves on the API/localhost host —
	// the page moves behind share.towncrierapp.uk in Slice 3 (#738). The canonical
	// path is always built from the slug + ref, never from the raw request host.
	shareOrigin = "https://share.towncrierapp.uk"

	// appleAppID is the Town Crier App Store numeric id. It drives the Smart App
	// Banner meta so iOS Safari offers "Open in app" / "Download".
	appleAppID = "6764095657"

	// App Store deep link, mirroring the web appStoreUrl('share-page') shape: the
	// campaign-free base, the share-page campaign token, and mt=8.
	appStoreBaseURL    = "https://apps.apple.com/gb/app/town-crier-planning-alerts/id6764095657"
	shareCampaignToken = "share-page"
	appStoreMediaType  = "8"

	// homeURL is the Town Crier marketing homepage. The share page has no
	// per-application web destination, so the always-present homepage link
	// (Problem 3, #763) points here regardless of device or which application is
	// being viewed.
	homeURL = "https://towncrierapp.uk"

	// ogDescriptionMaxRunes bounds the og:/twitter:description so a long proposal
	// does not overrun a social unfurl card. Counted in runes, not bytes, so a
	// multibyte character is never split.
	ogDescriptionMaxRunes = 200
)

// ctaURL is the sticky-CTA App Store link carrying the share-page campaign token,
// e.g. .../id6764095657?ct=share-page&mt=8.
func ctaURL() string {
	return appStoreBaseURL + "?ct=" + shareCampaignToken + "&mt=" + appStoreMediaType
}

// pageView is the pre-formatted, template-ready projection of a
// PlanningApplication. All optional sections are already resolved to empty
// strings / empty slices here so the template stays logic-light: an empty field
// means "omit this section". Dates are human-formatted in Go.
type pageView struct {
	Title         string
	OGTitle       string
	OGDescription string
	OGImage       string
	CanonicalURL  string
	AppleAppID    string
	CTAHref       string
	HomeURL       string

	Ref         string
	Address     string
	Postcode    string
	AppType     string
	StatusChip  string
	Description string
	Dates       []dateEntry
	PlanItLink  string
	CouncilLink string
}

// dateEntry is one row of the key-dates timeline (which doubles as the status
// history — the snapshot carries no richer feed).
type dateEntry struct {
	Label string
	Value string
}

// buildPageView projects an application into its template-ready view. slug and
// ref come from the resolved request path and are used verbatim to build the
// canonical URL. Every pointer field is nil-guarded: an absent value collapses
// to an empty string and the template omits the corresponding section.
func buildPageView(app applications.PlanningApplication, slug, ref string) pageView {
	place := strings.TrimSpace(app.Address)
	if place == "" {
		place = strings.TrimSpace(app.AreaName)
	}

	headline := ref
	if place != "" {
		headline = ref + " · " + place
	}

	canonical := shareOrigin + "/a/" + slug + "/" + ref

	v := pageView{
		Title:         headline + " · Town Crier",
		OGTitle:       headline,
		OGDescription: summarise(app.Description, place),
		// The unfurl image is the generated OSM map-card route for this exact
		// (slug, ref); that route decides map vs branded fallback, so the page
		// always points here (Slice 2, #738).
		OGImage:      shareOrigin + "/og/" + slug + "/" + ref + ".png",
		CanonicalURL: canonical,
		AppleAppID:   appleAppID,
		CTAHref:      ctaURL(),
		HomeURL:      homeURL,
		Ref:          ref,
		Address:      app.Address,
		Description:  app.Description,
	}

	if app.Postcode != nil && !addressIncludesPostcode(app.Address, *app.Postcode) {
		v.Postcode = *app.Postcode
	}
	if app.AppType != nil {
		v.AppType = *app.AppType
	}
	if app.AppState != nil {
		// Render the RAW PlanIt status verbatim — mapping it to a friendly label or
		// a status colour is a web/iOS concern, deliberately not invented here.
		v.StatusChip = *app.AppState
	}
	if app.StartDate != nil {
		v.Dates = append(v.Dates, dateEntry{Label: "Started", Value: formatDate(*app.StartDate)})
	}
	if app.ConsultedDate != nil {
		v.Dates = append(v.Dates, dateEntry{Label: "Consulted", Value: formatDate(*app.ConsultedDate)})
	}
	if app.DecidedDate != nil {
		v.Dates = append(v.Dates, dateEntry{Label: "Decided", Value: formatDate(*app.DecidedDate)})
	}
	if app.Link != nil {
		v.PlanItLink = *app.Link
	}
	if app.URL != nil {
		v.CouncilLink = *app.URL
	}
	return v
}

// addressIncludesPostcode reports whether address already ends with postcode,
// compared case-insensitively after trimming surrounding whitespace from
// both. PlanIt addresses commonly already carry the postcode as their tail
// (e.g. "2 High Street, Croydon, CR2 7DY"), so appending it again in the h1
// would render "... CR2 7DY, CR2 7DY" — the duplication this check guards
// against (tc-r4n9.6).
func addressIncludesPostcode(address, postcode string) bool {
	a := strings.TrimSpace(address)
	p := strings.TrimSpace(postcode)
	if a == "" || p == "" {
		return false
	}
	return strings.HasSuffix(strings.ToLower(a), strings.ToLower(p))
}

// formatDate renders a date as "2 March 2024".
func formatDate(t time.Time) string {
	return t.Format("2 January 2006")
}

// summarise builds the og:/twitter:description from the proposal text, trimmed to
// a concise length on a whole-rune boundary with an ellipsis. It falls back to a
// place-based sentence when the record carries no proposal text.
func summarise(description, place string) string {
	d := strings.TrimSpace(description)
	if d == "" {
		if place != "" {
			return "Planning application at " + place + ". View the details on Town Crier."
		}
		return "View this planning application on Town Crier."
	}
	runes := []rune(d)
	if len(runes) <= ogDescriptionMaxRunes {
		return d
	}
	return strings.TrimSpace(string(runes[:ogDescriptionMaxRunes])) + "…"
}
