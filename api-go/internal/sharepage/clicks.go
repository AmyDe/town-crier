package sharepage

import (
	"net/http"
	"net/url"
	"strings"
)

// SourceGroup is the closed set of origins a counted share-page visit is filed
// under. The share_page_click_totals table rejects any other value.
type SourceGroup string

// The source groups.
const (
	SourceFacebook SourceGroup = "facebook"
	SourceSEO      SourceGroup = "seo"
	SourceOther    SourceGroup = "other"
	SourceNone     SourceGroup = "none"
)

var crawlerAgentMarkers = []string{"bot", "crawl", "spider", "facebookexternalhit", "whatsapp"}

func (h *handler) countVisit(r *http.Request, slug, ref string) {
	if h.clicks == nil || r.Method != http.MethodGet || !likelyHuman(r) {
		return
	}
	h.clicks.Add(slug, ref, classifySource(r))
}

func likelyHuman(r *http.Request) bool {
	if r.Header.Get("Sec-Fetch-Mode") != "navigate" {
		return false
	}
	agent := strings.ToLower(r.UserAgent())
	for _, marker := range crawlerAgentMarkers {
		if strings.Contains(agent, marker) {
			return false
		}
	}
	return true
}

func classifySource(r *http.Request) SourceGroup {
	if r.URL.Query().Has("fbclid") {
		return SourceFacebook
	}
	referer := r.Referer()
	if referer == "" {
		return SourceNone
	}
	parsed, err := url.Parse(referer)
	if err != nil {
		return SourceOther
	}
	host := strings.ToLower(parsed.Hostname())
	switch {
	case host == "facebook.com", host == "fb.com", strings.HasSuffix(host, ".facebook.com"):
		return SourceFacebook
	case host == "towncrierapp.uk", strings.HasSuffix(host, ".towncrierapp.uk"):
		return SourceSEO
	default:
		return SourceOther
	}
}
