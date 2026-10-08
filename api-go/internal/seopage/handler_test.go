package seopage

import (
	"encoding/xml"
	"net/http"
	"strings"
	"testing"
)

func TestRoutes_NotFoundWithoutBuildKey(t *testing.T) {
	t.Parallel()
	mux := newTestMux(t, loadFixture(t))
	for _, path := range []string{"/planning", "/planning/cornwall", "/planning/cornwall/truro", "/sitemap.xml"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			resp := get(t, mux, path, false)
			if resp.Code != http.StatusNotFound || resp.Body != "" {
				t.Fatalf("keyless GET %s = %d with body %q, want bodyless 404", path, resp.Code, resp.Body)
			}
		})
	}
}

func TestRoutes_WrongBuildKeyIsNotFound(t *testing.T) {
	t.Parallel()
	mux := newTestMux(t, loadFixture(t))
	req := newKeyedRequest("/planning", "wrong-key")
	rec := recordTo(mux, req)
	if rec.Code != http.StatusNotFound || rec.Body.Len() != 0 {
		t.Fatalf("wrong key = %d with %d body bytes, want bodyless 404", rec.Code, rec.Body.Len())
	}
}

func TestRoutes_ServesPages(t *testing.T) {
	t.Parallel()
	mux := newTestMux(t, loadFixture(t))
	tests := []struct {
		name         string
		path         string
		wantH1       string
		wantModified string
	}{
		{"hub", "/planning", "Planning applications by council", "Sun, 28 Jun 2026 00:00:00 GMT"},
		{"towns index", "/planning/towns", "Planning applications by town", "Sun, 28 Jun 2026 00:00:00 GMT"},
		{"authority", "/planning/cornwall", "Planning applications in Cornwall", "Sun, 28 Jun 2026 00:00:00 GMT"},
		{"town", "/planning/cornwall/falmouth", "Planning applications in Falmouth", "Mon, 22 Jun 2026 00:00:00 GMT"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp := get(t, mux, tt.path, true)
			if resp.Code != http.StatusOK {
				t.Fatalf("GET %s = %d, want 200", tt.path, resp.Code)
			}
			if got := resp.Header.Get("Content-Type"); got != "text/html; charset=utf-8" {
				t.Errorf("Content-Type = %q", got)
			}
			if got := resp.Header.Get("Cache-Control"); got != "public, max-age=300" {
				t.Errorf("Cache-Control = %q", got)
			}
			if got := resp.Header.Get("Last-Modified"); got != tt.wantModified {
				t.Errorf("Last-Modified = %q, want %q", got, tt.wantModified)
			}
			if !strings.Contains(resp.Body, "<h1>"+tt.wantH1+"</h1>") {
				t.Errorf("body lacks h1 %q", tt.wantH1)
			}
		})
	}
}

func TestRoutes_TrailingSlashEqualsNoSlash(t *testing.T) {
	t.Parallel()
	mux := newTestMux(t, loadFixture(t))
	for _, path := range []string{"/planning", "/planning/towns", "/planning/cornwall", "/planning/cornwall/truro"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			plain := get(t, mux, path, true)
			slashed := get(t, mux, path+"/", true)
			if slashed.Code != http.StatusOK || slashed.Body != plain.Body {
				t.Fatalf("GET %s/ = %d (equal body: %t), want 200 identical to %s", path, slashed.Code, slashed.Body == plain.Body, path)
			}
		})
	}
}

func TestRoutes_RedirectRowIs301ToApexAuthority(t *testing.T) {
	t.Parallel()
	mux := newTestMux(t, loadFixture(t))
	resp := get(t, mux, "/planning/wrexham/wrexham", true)
	if resp.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want 301", resp.Code)
	}
	if got := resp.Header.Get("Location"); got != "https://towncrierapp.uk/planning/wrexham" {
		t.Errorf("Location = %q", got)
	}
	if got := resp.Header.Get("Cache-Control"); got != "public, max-age=300" {
		t.Errorf("Cache-Control = %q", got)
	}
}

func TestRoutes_UnknownPathIsBrandedNoindex404(t *testing.T) {
	t.Parallel()
	mux := newTestMux(t, loadFixture(t))
	for _, path := range []string{"/planning/does-not-exist", "/planning/cornwall/nowhere", "/planning/a/b/c"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			resp := get(t, mux, path, true)
			if resp.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", resp.Code)
			}
			if got := resp.Header.Get("Cache-Control"); got != "public, max-age=300" {
				t.Errorf("Cache-Control = %q", got)
			}
			for _, want := range []string{`<meta name="robots" content="noindex,follow" />`, "We couldn't find that page", "Town Crier"} {
				if !strings.Contains(resp.Body, want) {
					t.Errorf("404 body lacks %q", want)
				}
			}
			if strings.Contains(resp.Body, `rel="canonical"`) {
				t.Error("404 must not carry a canonical link")
			}
		})
	}
}

func TestRoutes_CanonicalIgnoresRequestHost(t *testing.T) {
	t.Parallel()
	mux := newTestMux(t, loadFixture(t))
	req := newKeyedRequest("/planning/cornwall", testBuildKey)
	req.Host = "preview.example.test"
	rec := recordTo(mux, req)
	if !strings.Contains(rec.Body.String(), `<link rel="canonical" href="https://towncrierapp.uk/planning/cornwall" />`) {
		t.Fatal("canonical must use the apex origin whatever the request host")
	}
}

type sitemapDoc struct {
	XMLName xml.Name `xml:"urlset"`
	URLs    []struct {
		Loc     string `xml:"loc"`
		Lastmod string `xml:"lastmod"`
	} `xml:"url"`
}

func TestRoutes_Sitemap(t *testing.T) {
	t.Parallel()
	fs := loadFixture(t)
	mux := newTestMux(t, fs)
	resp := get(t, mux, "/sitemap.xml", true)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	if got := resp.Header.Get("Content-Type"); got != "application/xml; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := resp.Header.Get("Cache-Control"); got != "public, max-age=300" {
		t.Errorf("Cache-Control = %q", got)
	}
	if !strings.HasPrefix(resp.Body, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n  <url>\n    <loc>https://towncrierapp.uk/planning</loc>\n    <lastmod>2026-06-28</lastmod>\n  </url>\n") {
		t.Errorf("unexpected sitemap prefix:\n%s", resp.Body[:min(300, len(resp.Body))])
	}
	var doc sitemapDoc
	if err := xml.Unmarshal([]byte(resp.Body), &doc); err != nil {
		t.Fatalf("sitemap is not valid XML: %v", err)
	}
	if len(doc.URLs) != len(fs.pages) {
		t.Fatalf("sitemap has %d urls, want %d", len(doc.URLs), len(fs.pages))
	}
	for i, p := range fs.pages {
		want := "https://towncrierapp.uk/planning"
		if p.Path != "" {
			want += "/" + p.Path
		}
		if doc.URLs[i].Loc != want {
			t.Errorf("url %d loc = %q, want %q (sort_order)", i, doc.URLs[i].Loc, want)
		}
	}
	if last := doc.URLs[len(doc.URLs)-1]; last.Loc != "https://towncrierapp.uk/planning/towns" || last.Lastmod != "2026-06-28" {
		t.Errorf("last url = %+v, want towns index with the newest town lastmod", last)
	}
	if got := resp.Header.Get("Last-Modified"); got != "Sun, 28 Jun 2026 00:00:00 GMT" {
		t.Errorf("Last-Modified = %q", got)
	}
}

func TestRoutes_SitemapOmitsMissingLastmod(t *testing.T) {
	t.Parallel()
	fs := loadFixture(t)
	fs.lastmod["cornwall/falmouth"] = nil
	resp := get(t, newTestMux(t, fs), "/sitemap.xml", true)
	var doc sitemapDoc
	if err := xml.Unmarshal([]byte(resp.Body), &doc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, u := range doc.URLs {
		if strings.HasSuffix(u.Loc, "/cornwall/falmouth") && u.Lastmod != "" {
			t.Errorf("falmouth lastmod = %q, want omitted", u.Lastmod)
		}
	}
}

func TestRoutes_StoreErrorIsBodyless500(t *testing.T) {
	t.Parallel()
	fs := loadFixture(t)
	fs.err = errStore
	mux := newTestMux(t, fs)
	for _, path := range []string{"/planning/cornwall", "/sitemap.xml"} {
		resp := get(t, mux, path, true)
		if resp.Code != http.StatusInternalServerError || resp.Body != "" {
			t.Errorf("GET %s = %d with body %q, want bodyless 500", path, resp.Code, resp.Body)
		}
	}
}

func TestRoutes_PageEscapesApplicationText(t *testing.T) {
	t.Parallel()
	resp := get(t, newTestMux(t, loadFixture(t)), "/planning/kings-lynn-and-west-norfolk", true)
	if strings.Contains(resp.Body, "<b>mill</b>") || strings.Contains(resp.Body, "<PE30 1AA>") {
		t.Error("application text must be HTML-escaped")
	}
	if !strings.Contains(resp.Body, "&lt;b&gt;mill&lt;/b&gt;") {
		t.Error("escaped application description missing")
	}
}

func TestRoutes_MidListCtaAfterEighthRow(t *testing.T) {
	t.Parallel()
	resp := get(t, newTestMux(t, loadFixture(t)), "/planning/basingstoke-and-deane", true)
	ctaAt := strings.Index(resp.Body, `class="ledgerCta"`)
	rows := strings.Count(resp.Body[:ctaAt], `class="ledgerRow"`)
	if ctaAt < 0 || rows != 8 {
		t.Fatalf("mid-list CTA after %d rows (found=%t), want after 8", rows, ctaAt >= 0)
	}
}
