package seopage

import (
	"strings"
	"time"
)

// SitemapEntry is one sitemap URL: a catalog page and the newest change to its
// application set (nil when the set is empty).
type SitemapEntry struct {
	Path    string
	Kind    string
	Lastmod *time.Time
}

func lastmodDay(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}

func renderSitemap(entries []SitemapEntry) string {
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")
	for i, e := range entries {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("  <url>\n    <loc>" + pagePath(e.Path) + "</loc>")
		if day := lastmodDay(e.Lastmod); day != "" {
			b.WriteString("\n    <lastmod>" + day + "</lastmod>")
		}
		b.WriteString("\n  </url>")
	}
	if len(entries) > 0 {
		b.WriteByte('\n')
	}
	b.WriteString("</urlset>\n")
	return b.String()
}
