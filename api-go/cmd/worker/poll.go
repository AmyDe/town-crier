package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/appevents"
	"github.com/AmyDe/town-crier/api-go/internal/metrics"
	"github.com/AmyDe/town-crier/api-go/internal/planit"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/polling"
	"github.com/AmyDe/town-crier/api-go/internal/worker"
)

// version is the build version sent in the PlanIt User-Agent. Set with
// -ldflags "-X main.version=<v>".
var version = "dev"

// pollRunnerAdapter runs one polling.Runner and logs its summary. It satisfies
// worker.PollRunner.
type pollRunnerAdapter struct {
	runner *polling.Runner
	logger *slog.Logger
}

func (a *pollRunnerAdapter) Run(ctx context.Context) error {
	res, err := a.runner.Run(ctx)
	if err != nil {
		return err
	}
	if !res.Acquired {
		a.logger.InfoContext(ctx, "poll run skipped: lease held")
		return nil
	}
	a.logger.InfoContext(ctx, "poll run completed",
		"pages", res.Pages, "callsToday", res.CallsToday, "stop", string(res.Stop),
		"health", string(res.Health.Level))
	return nil
}

// buildPoller wires the WORKER_MODE=poll runner: pacer, PlanIt client, window
// reader, ingester, event dispatcher, and the dev-only oracle.
func buildPoller(cfg platform.Config, pool *pgxpool.Pool, registry *metrics.Registry, st *stores, logger *slog.Logger) (*pollRunnerAdapter, error) {
	slots, err := parseDeltaSlots(cfg.PollingDeltaSlots)
	if err != nil {
		return nil, err
	}
	quietStart, err := appevents.ParseTimeOfDay(cfg.NotifyQuietStart)
	if err != nil {
		return nil, fmt.Errorf("parse NOTIFY_QUIET_START: %w", err)
	}
	quietEnd, err := appevents.ParseTimeOfDay(cfg.NotifyQuietEnd)
	if err != nil {
		return nil, fmt.Errorf("parse NOTIFY_QUIET_END: %w", err)
	}

	client, err := planit.NewClient(planit.Options{
		BaseURL: cfg.PlanItBaseURL,
		Metrics: registry,
		AreaID:  cfg.PollingAreaID,
		Version: version,
	})
	if err != nil {
		return nil, fmt.Errorf("build planit client: %w", err)
	}

	now := time.Now
	sw := polling.NewSwitch(polling.NewPostgresPollControlStore(pool), cfg.PollingEnabledDefault)
	pacer := polling.NewPacer(
		polling.NewPostgresPlanItCallStore(pool),
		polling.PacerConfig{
			DailyCap:   cfg.PollingDailyCallCap,
			MinSpacing: time.Duration(cfg.PollingMinRequestSpacingSeconds) * time.Second,
		},
		now, polling.SleepContext,
	).WithSwitch(sw)
	fetcher := polling.NewPacedWindowFetcher(pacer, client)

	decision, enqueuer, coalescer := buildNotifyFanOut(cfg, registry, st.zone, st, logger)
	enqueuer = enqueuer.WithMetrics(registry)
	decision = decision.WithMetrics(registry)
	dispatcher := appevents.NewDispatcher(
		appevents.NewPostgresStore(pool), st.app, decision, enqueuer, coalescer,
		appevents.Config{
			QuietStart:     quietStart,
			QuietEnd:       quietEnd,
			SurgeThreshold: cfg.NotifyEventSurgeThreshold,
			MaxAgeDays:     14,
		},
		now, logger,
	)

	windows := polling.NewPostgresWindowStore(pool)
	events := polling.NewPostgresPollEventStore(pool)
	oracleStore := polling.NewPostgresOracleStore(pool)
	var members polling.MemberStore
	if cfg.PollingOracleEnabled {
		members = oracleStore
	}
	observer := polling.NewRunObserver(polling.NewPostgresDeltaSeenStore(pool), members, events, dispatcher, now, logger)
	reader := polling.NewWindowReader(
		fetcher, polling.NewPostgresIngester(pool), windows,
		polling.WindowReadConfig{FullReadMaxAge: time.Duration(cfg.PollingFullReadMaxAgeDays) * 24 * time.Hour},
		now, logger,
	).WithHooks(observer)

	deps := polling.RunnerDeps{
		Switch:   sw,
		Lease:    st.lease,
		State:    windows,
		Calls:    pacer,
		Reader:   reader,
		Counters: observer,
		Push:     coalescer,
		Dispatch: observer,
		Flusher:  dispatcher,
		Health:   polling.NewPostgresHealthStore(pool),
		Now:      now,
		Log:      logger,
	}
	if cfg.PollingOracleEnabled {
		deps.Oracle = polling.NewOracle(fetcher, oracleStore, oracleStore, events, now, logger)
	}

	return &pollRunnerAdapter{
		runner: polling.NewRunner(polling.RunnerConfig{
			Plan: polling.WindowPlanConfig{
				DeltaSlots:    slots,
				DeltaMaxPages: cfg.PollingDeltaMaxPages,
				DailyCap:      cfg.PollingDailyCallCap,
				DayAllowance:  cfg.PollingDayAllowance,
			},
			RunBudget:     time.Duration(cfg.PollingRunBudgetMinutes) * time.Minute,
			OracleEnabled: cfg.PollingOracleEnabled,
		}, deps),
		logger: logger,
	}, nil
}

func parseDeltaSlots(raw string) ([]polling.CivilTime, error) {
	var slots []polling.CivilTime
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		slot, err := polling.ParseCivilTime(part)
		if err != nil {
			return nil, fmt.Errorf("parse POLLING_DELTA_SLOTS: %w", err)
		}
		slots = append(slots, slot)
	}
	return slots, nil
}

var _ worker.PollRunner = (*pollRunnerAdapter)(nil)
