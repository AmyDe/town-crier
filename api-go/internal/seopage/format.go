package seopage

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/AmyDe/town-crier/api-go/internal/seocatalog"
)

const (
	siteOrigin  = "https://towncrierapp.uk"
	shareOrigin = "https://share.towncrierapp.uk"

	appleAppID            = "6764095657"
	appStoreBaseURL       = "https://apps.apple.com/gb/app/town-crier-planning-alerts/id6764095657"
	appStoreProviderToken = "128810278"

	maxDescriptionRunes = 160
)

var attributionLines = []string{
	"Planning data provided by PlanIt (planit.org.uk)",
	"Contains public sector information licensed under the Open Government Licence. Crown Copyright.",
	"Contains Ordnance Survey data © Crown Copyright and database right.",
	"Map data © OpenStreetMap contributors.",
}

var townAttributionLines = append(append([]string(nil), attributionLines...),
	"Town locations contain Built-Up Areas (2022) data from the Office for National Statistics, licensed under the Open Government Licence. Crown Copyright.",
	"Scottish town locations contain data from National Records of Scotland, licensed under the Open Government Licence. Crown Copyright.",
)

func appStoreURL(campaign string) string {
	return appStoreBaseURL + "?pt=" + appStoreProviderToken + "&ct=" + escapeComponent(campaign) + "&mt=8"
}

func pagePath(path string) string {
	if path == "" {
		return siteOrigin + "/planning"
	}
	return siteOrigin + "/planning/" + path
}

// escapeComponent mirrors JavaScript's encodeURIComponent: every byte outside
// A-Z a-z 0-9 - _ . ! ~ * ' ( ) is percent-encoded.
func escapeComponent(s string) string {
	const unreserved = "-_.!~*'()"
	var b strings.Builder
	for i := range len(s) {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', strings.IndexByte(unreserved, c) >= 0:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// shareURL is the share-page URL for an application, or "" when either part is
// missing. The ref keeps its slashes (PlanIt references contain them).
func shareURL(authoritySlug, ref string) string {
	if authoritySlug == "" || ref == "" {
		return ""
	}
	parts := strings.Split(ref, "/")
	for i, p := range parts {
		parts[i] = escapeComponent(p)
	}
	return shareOrigin + "/a/" + authoritySlug + "/" + strings.Join(parts, "/")
}

var shortMonths = [...]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sept", "Oct", "Nov", "Dec"}

// formatDate renders a UTC date as "2 Jan 2006" with the en-GB short month
// names ("Sept" for September), matching the prerendered pages.
func formatDate(t time.Time) string {
	t = t.UTC()
	return fmt.Sprintf("%d %s %d", t.Day(), shortMonths[t.Month()-1], t.Year())
}

func truncate(text string, maxRunes int) string {
	if utf8.RuneCountInString(text) <= maxRunes {
		return text
	}
	cut := string([]rune(text)[:maxRunes])
	boundary := cut
	if i := strings.LastIndex(cut, " "); i > 0 {
		boundary = cut[:i]
	}
	return boundary + "…"
}

func statusDisplayLabel(appState *string) string {
	if appState == nil || *appState == "" {
		return "Unknown"
	}
	switch *appState {
	case "Permitted":
		return "Granted"
	case "Conditions":
		return "Granted with conditions"
	case "Rejected":
		return "Refused"
	default:
		return *appState
	}
}

func statusModifier(appState *string) string {
	if appState == nil {
		return "neutral"
	}
	switch *appState {
	case "Permitted":
		return "granted"
	case "Rejected":
		return "refused"
	default:
		return "neutral"
	}
}

type otherState struct {
	Label string
	Count int
}

type statusSummary struct {
	Granted, Refused, Undecided, Total int
	Other                              []otherState
	OtherTotal                         int
}

func aggregateStatus(breakdown []seocatalog.StateCount) statusSummary {
	var s statusSummary
	other := map[string]int{}
	for _, sc := range breakdown {
		s.Total += sc.Count
		switch {
		case sc.AppState != nil && *sc.AppState == "Permitted":
			s.Granted += sc.Count
		case sc.AppState != nil && *sc.AppState == "Rejected":
			s.Refused += sc.Count
		case sc.AppState == nil || *sc.AppState == "" || *sc.AppState == "Undecided":
			s.Undecided += sc.Count
		default:
			other[statusDisplayLabel(sc.AppState)] += sc.Count
		}
	}
	for label, count := range other {
		s.Other = append(s.Other, otherState{Label: label, Count: count})
		s.OtherTotal += count
	}
	sort.Slice(s.Other, func(i, j int) bool {
		if s.Other[i].Count != s.Other[j].Count {
			return s.Other[i].Count > s.Other[j].Count
		}
		return s.Other[i].Label < s.Other[j].Label
	})
	return s
}

func leadLine(area string, total int) string {
	noun := "planning applications"
	if total == 1 {
		noun = "planning application"
	}
	return fmt.Sprintf("See what's happening with planning in %s: %d %s tracked so far.", area, total, noun)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// collate orders names case-insensitively, falling back to the exact string,
// approximating the locale-aware ordering of the prerendered index pages.
func collate(a, b string) int {
	if c := strings.Compare(strings.ToLower(a), strings.ToLower(b)); c != 0 {
		return c
	}
	return strings.Compare(a, b)
}
