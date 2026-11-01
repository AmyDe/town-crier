package main

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
)

type applicationLocationsJobs interface {
	Load(ctx context.Context) (int64, error)
	Reconcile(ctx context.Context, all bool) (applications.ReconcileResult, error)
}

type applicationLocationsRunner struct{ jobs applicationLocationsJobs }

func (r applicationLocationsRunner) Load(ctx context.Context) (int64, error) {
	return r.jobs.Load(ctx)
}

func (r applicationLocationsRunner) Reconcile(ctx context.Context, all bool) (int, error) {
	res, err := r.jobs.Reconcile(ctx, all)
	if err != nil {
		return 0, err
	}
	if res.Failed > 0 {
		return res.Upserted + res.Deleted, errAuthoritiesAbandoned
	}
	return res.Upserted + res.Deleted, nil
}

func buildApplicationLocations(pool *pgxpool.Pool, logger *slog.Logger) applicationLocationsRunner {
	sw := applications.NewReadPathSwitch(applications.NewPostgresReadPathStore(pool))
	return applicationLocationsRunner{jobs: applications.NewApplicationLocations(pool, sw, logger, time.Now)}
}

var errAuthoritiesAbandoned = errors.New("reconcile abandoned one or more authorities")
