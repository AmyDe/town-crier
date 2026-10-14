package seopage

import (
	"encoding/json"
	"fmt"
	"html/template"
	"sort"
	"strings"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/seocatalog"
)

const (
	midListCTAAfter           = 8
	midListCTAMinApplications = 12
	notFoundTitle             = "Page not found | Town Crier"
)

type crumb struct {
	Label string
	Href  string
}

// layout carries everything the shared document head, masthead, breadcrumb and
// footer render. An empty Canonical omits the canonical and og:url tags.
type layout struct {
	Title           string
	MetaDescription string
	OGTitle         string
	Canonical       string
	Robots          string
	JSONLD          template.JS
	AppleAppID      string
	HeaderCTA       string
	Crumbs          []crumb
	TownsStyles     bool
	Year            int
	Attribution     []string
}

type linkView struct {
	Href string
	Text string
}

type appRow struct {
	Address     string
	Description string
	Ref         string
	DateLine    string
	Modifier    string
	Label       string
	ShareURL    string
}

type midCTA struct {
	Area string
	Href string
}

type ledgerItem struct {
	Row appRow
	CTA *midCTA
}

type listing struct {
	Layout      layout
	Area        string
	DataUpdated string
	Lead        string
	InlineHref  string
	Summary     statusSummary
	Items       []ledgerItem
	BottomHref  string
	QR          template.HTML
}

type authorityView struct {
	listing
	Towns      []linkView
	Neighbours []linkView
}

type townView struct {
	listing
	AuthorityName string
	AuthorityPath string
	Nearby        []linkView
}

type hubEntry struct {
	Href string
	Name string
	Meta string
}

type hubGroup struct {
	Letter  string
	Entries []hubEntry
}

type hubView struct {
	Layout    layout
	Count     int
	Noun      string
	StoreHref string
	Groups    []hubGroup
}

type townsIndexEntry struct {
	Href      string
	Name      string
	Authority string
}

type townsIndexSection struct {
	Letter  string
	Entries []townsIndexEntry
}

type townsIndexView struct {
	Layout    layout
	Count     int
	Noun      string
	StoreHref string
	Sections  []townsIndexSection
}

type notFoundView struct {
	Layout layout
}

func newLayout(now time.Time, attribution []string) layout {
	return layout{Robots: "index,follow", AppleAppID: appleAppID, Year: now.Year(), Attribution: attribution}
}

func jsonLD(docs ...any) (template.JS, error) {
	b, err := json.Marshal(docs)
	if err != nil {
		return "", fmt.Errorf("encode JSON-LD: %w", err)
	}
	return template.JS(b), nil //nolint:gosec // json.Marshal escapes <, > and & so the value cannot close the script element
}

func listItem(position int, name, item string) map[string]any {
	return map[string]any{"@type": "ListItem", "position": position, "name": name, "item": item}
}

func breadcrumbLD(crumbs ...map[string]any) map[string]any {
	items := make([]any, len(crumbs))
	for i, c := range crumbs {
		items[i] = c
	}
	return map[string]any{"@context": "https://schema.org", "@type": "BreadcrumbList", "itemListElement": items}
}

func datasetLD(name, description, canonical string) map[string]any {
	return map[string]any{
		"@context":            "https://schema.org",
		"@type":               "Dataset",
		"name":                name,
		"description":         description,
		"url":                 canonical,
		"isAccessibleForFree": true,
		"creator":             map[string]any{"@type": "Organization", "name": "PlanIt", "url": "https://planit.org.uk"},
		"license":             "https://www.nationalarchives.gov.uk/doc/open-government-licence/version/3/",
	}
}

func itemListLD(name, canonical, authoritySlug string, apps []applications.PlanningApplication) map[string]any {
	elements := make([]any, len(apps))
	for i := range apps {
		app := &apps[i]
		var label []string
		for _, s := range []string{app.Name, app.Address} {
			if s != "" {
				label = append(label, s)
			}
		}
		url := firstNonEmpty(shareURL(authoritySlug, app.Name), deref(app.URL), deref(app.Link), canonical)
		elements[i] = map[string]any{"@type": "ListItem", "position": i + 1, "name": strings.Join(label, " — "), "url": url}
	}
	return map[string]any{
		"@context":        "https://schema.org",
		"@type":           "ItemList",
		"name":            name,
		"url":             canonical,
		"numberOfItems":   len(apps),
		"itemListElement": elements,
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func applicationDateLine(app *applications.PlanningApplication) string {
	if app.DecidedDate != nil {
		return "Decided " + formatDate(*app.DecidedDate)
	}
	if app.StartDate != nil {
		return "Started " + formatDate(*app.StartDate) + " · Awaiting decision"
	}
	return ""
}

func buildLedger(apps []applications.PlanningApplication, authoritySlug, area, midHref string) []ledgerItem {
	items := make([]ledgerItem, 0, len(apps)+1)
	for i := range apps {
		app := &apps[i]
		items = append(items, ledgerItem{Row: appRow{
			Address:     app.Address,
			Description: truncate(app.Description, maxDescriptionRunes),
			Ref:         app.Name,
			DateLine:    applicationDateLine(app),
			Modifier:    statusModifier(app.AppState),
			Label:       statusDisplayLabel(app.AppState),
			ShareURL:    shareURL(authoritySlug, app.Name),
		}})
	}
	if len(apps) >= midListCTAMinApplications {
		cta := ledgerItem{CTA: &midCTA{Area: area, Href: midHref}}
		items = append(items[:midListCTAAfter], append([]ledgerItem{cta}, items[midListCTAAfter:]...)...)
	}
	return items
}

func dataUpdatedLine(lastmod *time.Time) string {
	if lastmod == nil {
		return ""
	}
	return "Last application update " + formatDate(*lastmod)
}

func newListing(l layout, area, authoritySlug, campaignPrefix string, page seocatalog.Page, apps []applications.PlanningApplication, lastmod *time.Time) (listing, error) {
	qrCode, err := qrSVG(appStoreURL(campaignPrefix+"-qr"), "QR code linking to Town Crier on the App Store")
	if err != nil {
		return listing{}, err
	}
	return listing{
		Layout:      l,
		Area:        area,
		DataUpdated: dataUpdatedLine(lastmod),
		Lead:        leadLine(area, page.Total),
		InlineHref:  appStoreURL(campaignPrefix + "-inline"),
		Summary:     aggregateStatus(page.StatusBreakdown),
		Items:       buildLedger(apps, authoritySlug, area, appStoreURL(campaignPrefix+"-mid")),
		BottomHref:  appStoreURL(campaignPrefix + "-btm"),
		QR:          qrCode,
	}, nil
}

func buildAuthorityView(now time.Time, page seocatalog.Page, apps []applications.PlanningApplication, lastmod *time.Time) (authorityView, error) {
	slug := page.Path
	area := page.DisplayName
	canonical := pagePath(slug)
	desc := fmt.Sprintf("Recent planning applications in %s. See what is being built nearby and get push alerts the moment an application is submitted or decided in your area.", area)
	ld, err := jsonLD(
		itemListLD("Planning applications in "+area, canonical, slug, apps),
		datasetLD("Recent planning applications in "+area,
			fmt.Sprintf("Recent local planning applications in %s, drawn from Town Crier's planning-application snapshot.", area), canonical),
		breadcrumbLD(listItem(1, "Town Crier", siteOrigin+"/"), listItem(2, "Planning applications", siteOrigin+"/planning"), listItem(3, area, canonical)),
	)
	if err != nil {
		return authorityView{}, err
	}
	l := newLayout(now, attributionLines)
	l.Title = "Planning applications in " + area + " | Town Crier"
	l.MetaDescription = desc
	l.OGTitle = "Planning applications in " + area
	l.Canonical = canonical
	l.JSONLD = ld
	l.HeaderCTA = appStoreURL("seo-lpa-hdr")
	l.Crumbs = []crumb{{"Town Crier", "/"}, {"Planning applications", "/planning"}, {area, ""}}

	li, err := newListing(l, area, slug, "seo-lpa", page, apps, lastmod)
	if err != nil {
		return authorityView{}, err
	}
	v := authorityView{listing: li}
	for _, t := range page.Children {
		v.Towns = append(v.Towns, linkView{Href: "/planning/" + slug + "/" + t.Slug, Text: t.Name})
	}
	for _, n := range page.Neighbours {
		v.Neighbours = append(v.Neighbours, linkView{Href: "/planning/" + n.Slug, Text: n.Name})
	}
	return v, nil
}

func buildTownView(now time.Time, page seocatalog.Page, apps []applications.PlanningApplication, lastmod *time.Time) (townView, error) {
	authoritySlug, _, _ := strings.Cut(page.Path, "/")
	town := page.DisplayName
	authority := page.AuthorityName
	canonical := pagePath(page.Path)
	authorityCanonical := pagePath(authoritySlug)
	desc := fmt.Sprintf("Recent planning applications in %s, %s. See what is being built nearby and get push alerts the moment an application is submitted or decided near you.", town, authority)
	ld, err := jsonLD(
		itemListLD("Planning applications in "+town, canonical, authoritySlug, apps),
		datasetLD("Recent planning applications in "+town,
			fmt.Sprintf("Recent local planning applications in and around %s, %s, drawn from Town Crier's planning-application snapshot.", town, authority), canonical),
		breadcrumbLD(listItem(1, "Town Crier", siteOrigin+"/"), listItem(2, "Planning applications", siteOrigin+"/planning"),
			listItem(3, authority, authorityCanonical), listItem(4, town, canonical)),
	)
	if err != nil {
		return townView{}, err
	}
	l := newLayout(now, townAttributionLines)
	l.Title = "Planning applications in " + town + " | Town Crier"
	l.MetaDescription = desc
	l.OGTitle = "Planning applications in " + town
	l.Canonical = canonical
	l.JSONLD = ld
	l.HeaderCTA = appStoreURL("seo-town-hdr")
	authorityPath := "/planning/" + authoritySlug
	l.Crumbs = []crumb{{"Town Crier", "/"}, {"Planning applications", "/planning"}, {authority, authorityPath}, {town, ""}}

	li, err := newListing(l, town, authoritySlug, "seo-town", page, apps, lastmod)
	if err != nil {
		return townView{}, err
	}
	v := townView{listing: li, AuthorityName: authority, AuthorityPath: authorityPath}
	for _, n := range page.Neighbours {
		v.Nearby = append(v.Nearby, linkView{Href: "/planning/" + n.AuthoritySlug + "/" + n.Slug, Text: n.Name + ", " + n.AuthorityName})
	}
	return v, nil
}

func buildHubView(now time.Time, authorityPages []seocatalog.Page) (hubView, error) {
	pages := append([]seocatalog.Page(nil), authorityPages...)
	sort.SliceStable(pages, func(i, j int) bool { return collate(pages[i].DisplayName, pages[j].DisplayName) < 0 })

	canonical := pagePath("")
	count := len(pages)
	noun := plural(count, "local planning authority", "local planning authorities")
	items := make([]any, count)
	var groups []hubGroup
	for i := range pages {
		p := &pages[i]
		items[i] = map[string]any{"@type": "ListItem", "position": i + 1, "name": p.DisplayName, "url": pagePath(p.Path)}
		letter := "#"
		if name := strings.TrimSpace(p.DisplayName); name != "" {
			letter = strings.ToUpper(string([]rune(name)[:1]))
		}
		meta := fmt.Sprintf("%d %s tracked", p.Total, plural(p.Total, "application", "applications"))
		if n := len(p.Children); n > 0 {
			meta += fmt.Sprintf(" · %d %s", n, plural(n, "town", "towns"))
		}
		entry := hubEntry{Href: "/planning/" + p.Path, Name: p.DisplayName, Meta: meta}
		if len(groups) > 0 && groups[len(groups)-1].Letter == letter {
			groups[len(groups)-1].Entries = append(groups[len(groups)-1].Entries, entry)
		} else {
			groups = append(groups, hubGroup{Letter: letter, Entries: []hubEntry{entry}})
		}
	}
	ld, err := jsonLD(
		map[string]any{
			"@context": "https://schema.org", "@type": "ItemList", "name": "UK local planning authorities",
			"url": canonical, "numberOfItems": count, "itemListElement": items,
		},
		breadcrumbLD(listItem(1, "Town Crier", siteOrigin+"/"), listItem(2, "Planning applications", canonical)),
	)
	if err != nil {
		return hubView{}, err
	}
	store := appStoreURL("seo-hub")
	l := newLayout(now, attributionLines)
	l.Title = "Planning applications by council | Town Crier"
	l.MetaDescription = fmt.Sprintf("Browse recent planning applications for %d %s across the UK. Find your council and get push alerts the moment a new application is submitted or decided.", count, noun)
	l.OGTitle = "Planning applications by council"
	l.Canonical = canonical
	l.JSONLD = ld
	l.HeaderCTA = store
	l.Crumbs = []crumb{{"Town Crier", "/"}, {"Planning applications", ""}}
	return hubView{Layout: l, Count: count, Noun: noun, StoreHref: store, Groups: groups}, nil
}

func townIndexLetter(name string) string {
	r := []rune(strings.ToUpper(strings.TrimSpace(name)))
	if len(r) > 0 && r[0] >= 'A' && r[0] <= 'Z' {
		return string(r[:1])
	}
	return "#"
}

func buildTownsIndexView(now time.Time, townPages []seocatalog.Page) (townsIndexView, error) {
	type entry struct {
		townName, authorityName, path string
	}
	buckets := map[string][]entry{}
	for i := range townPages {
		p := &townPages[i]
		letter := townIndexLetter(p.DisplayName)
		buckets[letter] = append(buckets[letter], entry{p.DisplayName, p.AuthorityName, p.Path})
	}
	letters := make([]string, 0, len(buckets))
	for letter := range buckets {
		letters = append(letters, letter)
	}
	sort.Slice(letters, func(i, j int) bool {
		if letters[i] == "#" || letters[j] == "#" {
			return letters[j] == "#" && letters[i] != "#"
		}
		return letters[i] < letters[j]
	})
	sections := make([]townsIndexSection, 0, len(letters))
	for _, letter := range letters {
		entries := buckets[letter]
		sort.SliceStable(entries, func(i, j int) bool {
			if c := collate(entries[i].townName, entries[j].townName); c != 0 {
				return c < 0
			}
			return collate(entries[i].authorityName, entries[j].authorityName) < 0
		})
		sec := townsIndexSection{Letter: letter}
		for _, e := range entries {
			sec.Entries = append(sec.Entries, townsIndexEntry{Href: "/planning/" + e.path, Name: e.townName, Authority: e.authorityName})
		}
		sections = append(sections, sec)
	}

	canonical := pagePath(seocatalog.TownsIndexSlug)
	ld, err := jsonLD(breadcrumbLD(listItem(1, "Town Crier", siteOrigin+"/"), listItem(2, "Towns", canonical)))
	if err != nil {
		return townsIndexView{}, err
	}
	cta := appStoreURL("seo-towns-index")
	l := newLayout(now, townAttributionLines)
	l.Title = "Planning applications by town | Town Crier"
	l.MetaDescription = "Browse recent planning applications by town across England, Wales and Scotland. Pick a town to see what is being built nearby, or get push alerts the moment something changes."
	l.OGTitle = "Planning applications by town"
	l.Canonical = canonical
	l.JSONLD = ld
	l.HeaderCTA = cta
	l.Crumbs = []crumb{{"Town Crier", "/"}, {"Towns", ""}}
	l.TownsStyles = true
	count := len(townPages)
	return townsIndexView{Layout: l, Count: count, Noun: plural(count, "town", "towns"), StoreHref: cta, Sections: sections}, nil
}

func buildNotFoundView(now time.Time) notFoundView {
	l := newLayout(now, attributionLines)
	l.Title = notFoundTitle
	l.Robots = "noindex,follow"
	l.HeaderCTA = appStoreURL("seo-404-hdr")
	l.Crumbs = []crumb{{"Town Crier", "/"}, {"Planning applications", "/planning"}}
	return notFoundView{Layout: l}
}
