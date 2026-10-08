package seopage

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

type headFields struct {
	Title       string
	Description string
	Canonical   string
	Robots      string
	OG          map[string]string
	H1          string
	JSONLD      any
}

func parseHead(t *testing.T, doc []byte) headFields {
	t.Helper()
	root, err := html.Parse(bytes.NewReader(doc))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}
	f := headFields{OG: map[string]string{}}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "title":
				f.Title = text(n)
			case "h1":
				if f.H1 == "" {
					f.H1 = text(n)
				}
			case "meta":
				name, prop, content := attr(n, "name"), attr(n, "property"), attr(n, "content")
				switch {
				case name == "description":
					f.Description = content
				case name == "robots":
					f.Robots = content
				case strings.HasPrefix(prop, "og:"):
					f.OG[prop] = content
				}
			case "link":
				if attr(n, "rel") == "canonical" {
					f.Canonical = attr(n, "href")
				}
			case "script":
				if attr(n, "type") == "application/ld+json" {
					if err := json.Unmarshal([]byte(text(n)), &f.JSONLD); err != nil {
						t.Fatalf("JSON-LD is not valid JSON: %v", err)
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return f
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func text(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(b.String())
}

func TestRender_HeadParityWithPrerender(t *testing.T) {
	t.Parallel()
	fs := loadFixture(t)
	mux := newTestMux(t, fs)

	type pair struct{ route, golden string }
	pairs := []pair{{"/planning", "hub.html"}, {"/planning/towns", "towns.html"}}
	for _, p := range fs.pages {
		switch p.Kind {
		case "authority":
			pairs = append(pairs, pair{"/planning/" + p.Path, "authority-" + p.Path + ".html"})
		case "town":
			pairs = append(pairs, pair{"/planning/" + p.Path, "town-" + strings.ReplaceAll(p.Path, "/", "-") + ".html"})
		}
	}
	if len(pairs) < 8 {
		t.Fatalf("only %d golden pairs; expected hub, towns, authorities and towns", len(pairs))
	}

	for _, p := range pairs {
		t.Run(p.golden, func(t *testing.T) {
			t.Parallel()
			want, err := os.ReadFile(filepath.Join("testdata", "golden", p.golden))
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			resp := get(t, mux, p.route, true)
			if resp.Code != 200 {
				t.Fatalf("GET %s = %d", p.route, resp.Code)
			}
			got, exp := parseHead(t, []byte(resp.Body)), parseHead(t, want)
			if exp.Title == "" || exp.Canonical == "" || exp.H1 == "" || exp.JSONLD == nil || len(exp.OG) != 5 {
				t.Fatalf("golden %s is missing head fields: %+v", p.golden, exp)
			}
			if !reflect.DeepEqual(got, exp) {
				gotJSON, _ := json.MarshalIndent(got, "", "  ")
				expJSON, _ := json.MarshalIndent(exp, "", "  ")
				t.Fatalf("head differs from prerender golden\n got: %s\nwant: %s", gotJSON, expJSON)
			}
		})
	}
}
