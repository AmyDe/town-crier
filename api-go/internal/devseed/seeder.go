// Package devseed mirrors a small slice of recently-changed prod planning
// applications into dev so a TestFlight build pointed at dev gets real push
// notifications to test against. Dev deliberately runs no PlanIt poller
// (ADR 0024) — this package sources its candidate applications from a
// read-only prod mirror instead, and feeds each one through the same
// polling.Ingester the real poll cycle uses (bd tc-grvu.4, GH#808).
package devseed

import (
	"context"
	"log/slog"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/polling"
)

// authorityLister lists the distinct authority ids dev currently has watch
// zones in. *watchzones.PostgresStore (dev-bound) satisfies it.
type authorityLister interface {
	DistinctAuthorityIDs(ctx context.Context) ([]int, error)
}

// prodReader reads the most-recently-changed prod applications scoped to a
// set of authority ids. *applications.PostgresStore, bound to a read-only
// prod pool, satisfies it.
type prodReader interface {
	RecentInAuthorities(ctx context.Context, authorityIDs []int, limit int) ([]applications.PlanningApplication, error)
}

// pushFlusher drives the poll-cycle push coalescer's lifecycle, the same
// contract polling.PollPlanItHandler uses (GH#784): Reset clears any pushes
// queued by a prior cycle, Flush sends this cycle's coalesced pushes.
// *notifydispatch.PushCoalescer satisfies it structurally.
type pushFlusher interface {
	Reset()
	Flush(ctx context.Context) error
}

// Seeder runs one dev-seed cycle: read dev's watched authorities, pull the
// most-recently-changed prod applications within them, and feed each through
// the real ingest pipeline.
type Seeder struct {
	zones    authorityLister
	prodApps prodReader
	ingester *polling.Ingester
	push     pushFlusher
	limit    int
	logger   *slog.Logger
}

// NewSeeder wires a Seeder.
func NewSeeder(zones authorityLister, prodApps prodReader, ingester *polling.Ingester, push pushFlusher, limit int, logger *slog.Logger) *Seeder {
	return &Seeder{
		zones:    zones,
		prodApps: prodApps,
		ingester: ingester,
		push:     push,
		limit:    limit,
		logger:   logger,
	}
}

// Run executes one dev-seed cycle. Zero watch zones is a normal state (a
// fresh/empty dev environment, or one where the last zone was just deleted),
// not an error: it is logged at info level and returns (0, nil) without
// touching the prod reader or the push coalescer.
func (s *Seeder) Run(ctx context.Context) (int, error) {
	authorityIDs, err := s.zones.DistinctAuthorityIDs(ctx)
	if err != nil {
		return 0, err
	}
	if len(authorityIDs) == 0 {
		s.logger.InfoContext(ctx, "dev-seed: no watched authorities, skipping cycle")
		return 0, nil
	}

	apps, err := s.prodApps.RecentInAuthorities(ctx, authorityIDs, s.limit)
	if err != nil {
		return 0, err
	}

	s.push.Reset()

	count := 0
	for _, app := range apps {
		if err := s.ingester.Ingest(ctx, app); err != nil {
			s.logger.ErrorContext(ctx, "dev-seed: ingest failed, continuing with remaining apps",
				"uid", app.UID, "authorityId", app.AreaID, "error", err)
			continue
		}
		count++
	}

	// A push flush problem must never fail the cycle, mirroring
	// polling.PollPlanItHandler.Handle's own posture around its Flush call.
	if err := s.push.Flush(ctx); err != nil {
		s.logger.ErrorContext(ctx, "dev-seed: push flush failed", "error", err)
	}

	return count, nil
}
