package main

import (
	"context"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/indexnow"
	"github.com/AmyDe/town-crier/api-go/internal/seopage"
	"github.com/AmyDe/town-crier/api-go/internal/worker"
)

type stubSubmitter struct{}

func (stubSubmitter) Submit(context.Context) (indexnow.Result, error) {
	return indexnow.Result{Submitted: 2, Skipped: 3}, nil
}

func TestNewSEOCatalogRunner_IndexNowOnlyInProd(t *testing.T) {
	t.Parallel()
	tests := []struct {
		env  string
		want bool
	}{
		{"prod", true},
		{"dev", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run("env="+tt.env, func(t *testing.T) {
			t.Parallel()
			r := newSEOCatalogRunner(seoCatalogRunner{}, tt.env, stubSubmitter{})
			_, got := r.(worker.IndexNowSubmitter)
			if got != tt.want {
				t.Errorf("implements IndexNowSubmitter = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIndexNowSubmit_ReportsCounts(t *testing.T) {
	t.Parallel()
	r := indexNowSeoRunner{seoCatalogRunner: seoCatalogRunner{}, submitter: stubSubmitter{}}
	submitted, skipped, err := r.SubmitIndexNow(context.Background())
	if err != nil || submitted != 2 || skipped != 3 {
		t.Errorf("got %d, %d, %v; want 2, 3, nil", submitted, skipped, err)
	}
}

type fakeSitemapReader struct{ entries []seopage.SitemapEntry }

func (f fakeSitemapReader) Sitemap(context.Context) ([]seopage.SitemapEntry, error) {
	return f.entries, nil
}

func TestSitemapPairSource_ConvertsPairs(t *testing.T) {
	t.Parallel()
	src := sitemapPairSource{reader: fakeSitemapReader{entries: []seopage.SitemapEntry{{Path: "", Kind: "hub"}, {Path: "cornwall", Kind: "authority"}}}}
	pairs, err := src.Pairs(context.Background())
	if err != nil || len(pairs) != 2 || pairs[1].Loc != "https://towncrierapp.uk/planning/cornwall" {
		t.Errorf("pairs = %v, err %v", pairs, err)
	}
}
