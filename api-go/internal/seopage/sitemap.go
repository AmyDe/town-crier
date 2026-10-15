package seopage

import (
	"context"
	"strings"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/seocatalog"
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

// SitemapReader is the store read the sitemap is built from.
type SitemapReader interface {
	Sitemap(ctx context.Context) ([]SitemapEntry, error)
}

// SitemapPair is one sitemap URL as crawlers see it: the absolute location and
// the formatted lastmod day ("" when the page has none).
type SitemapPair struct {
	Loc     string
	Lastmod string
}

// SitemapPairs returns the (loc, lastmod) pairs /sitemap.xml serves, in sitemap
// order, using the same lastmod rollup as the handler.
func SitemapPairs(ctx context.Context, r SitemapReader) ([]SitemapPair, error) {
	entries, err := rolledUpSitemap(ctx, r)
	if err != nil {
		return nil, err
	}
	pairs := make([]SitemapPair, len(entries))
	for i, e := range entries {
		pairs[i] = SitemapPair{Loc: pagePath(e.Path), Lastmod: lastmodDay(e.Lastmod)}
	}
	return pairs, nil
}

// rolledUpSitemap reads the entries and gives the hub and towns-index rows the
// newest lastmod of the pages they list.
func rolledUpSitemap(ctx context.Context, r SitemapReader) ([]SitemapEntry, error) {
	entries, err := r.Sitemap(ctx)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		switch entries[i].Kind {
		case seocatalog.KindHub:
			entries[i].Lastmod = newest(entries, "")
		case seocatalog.KindTowns:
			entries[i].Lastmod = newest(entries, seocatalog.KindTown)
		}
	}
	return entries, nil
}
