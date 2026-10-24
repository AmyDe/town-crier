package sharepage

import (
	"strings"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
)

const (
	// Canonical and og:url are built from shareOrigin plus slug and ref, never
	// from the request host.
	shareOrigin = "https://share.towncrierapp.uk"

	// appleAppID is the Town Crier App Store numeric id. It drives the Smart App
	// Banner meta so iOS Safari offers "Open in app" / "Download".
	appleAppID = "6764095657"

	// Mirrors web appStoreUrl('share-page'). Apple silently drops ct from App
	// Analytics unless the provider token (pt) accompanies it.
	appStoreBaseURL       = "https://apps.apple.com/gb/app/town-crier-planning-alerts/id" + appleAppID
	appStoreProviderToken = "128810278"
	shareCampaignToken    = "share-page"
	appStoreMediaType     = "8"

	homeURL = "https://towncrierapp.uk"

	// ogDescriptionMaxRunes bounds the og:/twitter:description so a long proposal
	// does not overrun a social unfurl card. Counted in runes, not bytes, so a
	// multibyte character is never split.
	ogDescriptionMaxRunes = 200
)

// ctaURL is the sticky-CTA App Store link carrying the provider and share-page
// campaign tokens, e.g. .../id6764095657?pt=128810278&ct=share-page&mt=8.
func ctaURL() string {
	return appStoreBaseURL + "?pt=" + appStoreProviderToken +
		"&ct=" + shareCampaignToken + "&mt=" + appStoreMediaType
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

	Ref            string
	Address        string
	Postcode       string
	AppType        string
	StatusLabel    string
	StatusModifier string
	Description    string
	Dates          []dateEntry
	PlanItLink     string
	CouncilLink    string
	AuthorityName  string
	AuthorityURL   string
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
		OGImage:       shareOrigin + "/og/" + slug + "/" + ref + ".png",
		CanonicalURL:  canonical,
		AppleAppID:    appleAppID,
		CTAHref:       ctaURL(),
		HomeURL:       homeURL,
		Ref:           ref,
		Address:       app.Address,
		Description:   app.Description,
	}

	if app.Postcode != nil && !addressIncludesPostcode(app.Address, *app.Postcode) {
		v.Postcode = *app.Postcode
	}
	if app.AppType != nil {
		v.AppType = *app.AppType
	}
	if app.AppState != nil {
		v.StatusLabel, v.StatusModifier = statusChip(*app.AppState)
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
	if areaName := strings.TrimSpace(app.AreaName); areaName != "" {
		v.AuthorityName = areaName
		v.AuthorityURL = homeURL + "/planning/" + slug
	}
	return v
}

// statusLabels mirrors STATUS_DISPLAY_LABEL_MAP in web/scripts/lib/format.mjs.
var statusLabels = map[string]string{
	"Permitted":  "Granted",
	"Conditions": "Granted with conditions",
	"Rejected":   "Refused",
}

// statusModifiers deliberately colours "Conditions" neutral, not green,
// matching the web bucketing of long-tail states.
var statusModifiers = map[string]string{
	"Permitted":  "granted",
	"Conditions": "neutral",
	"Rejected":   "refused",
}

// statusChip translates a raw PlanIt appState into its (label, CSS modifier)
// pair. An unrecognised appState renders as itself with the neutral modifier,
// so an unmapped PlanIt string is still visible rather than silently dropped.
func statusChip(appState string) (label, modifier string) {
	if l, ok := statusLabels[appState]; ok {
		return l, statusModifiers[appState]
	}
	return appState, "neutral"
}

// addressIncludesPostcode guards against rendering the postcode twice, since
// PlanIt addresses often already end with it.
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
