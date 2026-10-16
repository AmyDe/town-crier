package seopage

import (
	"context"
	"encoding/xml"
	"errors"
	"testing"
)

func TestSitemapPairs_MatchServedSitemap(t *testing.T) {
	t.Parallel()
	fs := loadFixture(t)
	fs.lastmod["cornwall/falmouth"] = nil

	pairs, err := SitemapPairs(context.Background(), fs)
	if err != nil {
		t.Fatalf("SitemapPairs: %v", err)
	}

	resp := get(t, newTestMux(t, fs), "/sitemap.xml", true)
	var doc sitemapDoc
	if err := xml.Unmarshal([]byte(resp.Body), &doc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(pairs) != len(doc.URLs) {
		t.Fatalf("pairs = %d, served urls = %d", len(pairs), len(doc.URLs))
	}
	for i, u := range doc.URLs {
		if pairs[i].Loc != u.Loc || pairs[i].Lastmod != u.Lastmod {
			t.Errorf("pair %d = %+v, served (%q, %q)", i, pairs[i], u.Loc, u.Lastmod)
		}
	}
}

func TestSitemapPairs_PropagatesStoreError(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	if _, err := SitemapPairs(context.Background(), &fakeStore{err: boom}); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want wrapping boom", err)
	}
}
