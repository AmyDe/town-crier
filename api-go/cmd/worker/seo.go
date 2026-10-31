package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/authorities"
	"github.com/AmyDe/town-crier/api-go/internal/indexnow"
	"github.com/AmyDe/town-crier/api-go/internal/seocatalog"
	"github.com/AmyDe/town-crier/api-go/internal/seopage"
	"github.com/AmyDe/town-crier/api-go/internal/worker"
)

const (
	indexNowHost = "towncrierapp.uk"
	indexNowKey  = "1cb201daee1a485fb8cce2a68e8ee465"
	prodEnv      = "prod"
)

type seoAdapter struct {
	store  *seocatalog.Store
	logger *slog.Logger
}

func buildSEO(pool *pgxpool.Pool, logger *slog.Logger) (*seoAdapter, error) {
	gazetteer, err := seocatalog.EmbeddedGazetteer()
	if err != nil {
		return nil, fmt.Errorf("load town gazetteer: %w", err)
	}
	store := seocatalog.NewStore(pool, gazetteer, authorities.NewLookup())
	return &seoAdapter{store: store, logger: logger}, nil
}

type seoCatalogRunner struct{ *seoAdapter }

func (a seoCatalogRunner) Run(ctx context.Context) (int, int, error) {
	sum, err := a.store.Rebuild(ctx)
	if err != nil {
		return 0, 0, err
	}
	a.logger.InfoContext(ctx, "seo-catalog completed",
		"authorities", sum.Authorities, "towns", sum.Towns, "redirects", sum.Redirects)
	return sum.Authorities, sum.Towns, nil
}

type indexNowSubmitter interface {
	Submit(ctx context.Context) (indexnow.Result, error)
}

type indexNowSeoRunner struct {
	seoCatalogRunner
	submitter indexNowSubmitter
}

func (r indexNowSeoRunner) SubmitIndexNow(ctx context.Context) (int, int, error) {
	res, err := r.submitter.Submit(ctx)
	return res.Submitted, res.Skipped, err
}

// newSEOCatalogRunner returns the catalog runner, with IndexNow submission only
// in prod: dev must never notify search engines.
func newSEOCatalogRunner(base seoCatalogRunner, env string, submitter indexNowSubmitter) worker.SEOCatalogRunner {
	if env != prodEnv {
		return base
	}
	return indexNowSeoRunner{seoCatalogRunner: base, submitter: submitter}
}

// sitemapPairSource feeds the IndexNow diff from the same sitemap build the
// live /sitemap.xml serves.
type sitemapPairSource struct{ reader seopage.SitemapReader }

func (s sitemapPairSource) Pairs(ctx context.Context) ([]indexnow.Pair, error) {
	pairs, err := seopage.SitemapPairs(ctx, s.reader)
	if err != nil {
		return nil, err
	}
	out := make([]indexnow.Pair, len(pairs))
	for i, p := range pairs {
		out[i] = indexnow.Pair{Loc: p.Loc, Lastmod: p.Lastmod}
	}
	return out, nil
}

func buildIndexNow(pool *pgxpool.Pool, logger *slog.Logger) *indexnow.Submitter {
	store := seopage.NewPostgresStore(pool, applications.NewPostgresStore(pool))
	return indexnow.New(
		sitemapPairSource{reader: store},
		indexnow.NewPostgresStateStore(pool),
		indexnow.Config{Host: indexNowHost, Key: indexNowKey},
		logger,
	)
}
