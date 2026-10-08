package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/authorities"
	"github.com/AmyDe/town-crier/api-go/internal/seocatalog"
)

// seoAdapter exposes the seocatalog store as the worker's SEO runners.
type seoAdapter struct {
	store  *seocatalog.Store
	logger *slog.Logger
}

// buildSEO wires the seo-assign and seo-catalog runners over the embedded
// gazetteer and authority list.
func buildSEO(pool *pgxpool.Pool, logger *slog.Logger) (*seoAdapter, error) {
	gazetteer, err := seocatalog.EmbeddedGazetteer()
	if err != nil {
		return nil, fmt.Errorf("load town gazetteer: %w", err)
	}
	store := seocatalog.NewStore(pool, gazetteer, authorities.NewLookup())
	return &seoAdapter{store: store, logger: logger}, nil
}

type seoAssignRunner struct{ *seoAdapter }

func (a seoAssignRunner) Run(ctx context.Context) (int, error) {
	sum, err := a.store.Assign(ctx)
	if err != nil {
		return 0, err
	}
	a.logger.InfoContext(ctx, "seo-assign completed",
		"batches", sum.Batches, "processed", sum.Processed, "gazetteerReset", sum.Reset)
	return sum.Processed, nil
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
