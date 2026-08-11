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
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// zoneLister lists every watch zone dev currently has, across every user.
// *watchzones.PostgresStore (dev-bound) satisfies it. It replaces the
// authority-scoped authorityLister (bd tc-9nbs4.1, GH#1076 Phase 1): dev's
// watch zones no longer need a "home" authority to scope the prod read, since
// notification matching is purely geographic (ADR 0041/0044).
type zoneLister interface {
	All(ctx context.Context) ([]watchzones.WatchZone, error)
}

// prodReader reads the most-recently-changed prod applications that fall
// within a set of watch-zone geometries. *applications.PostgresStore, bound
// to a read-only prod pool, satisfies it.
type prodReader interface {
	RecentNearZones(ctx context.Context, zones []applications.ZoneGeometry, limit int) ([]applications.PlanningApplication, error)
}

// pushFlusher drives the poll-cycle push coalescer's lifecycle, the same
// contract polling.PollPlanItHandler uses (GH#784): Reset clears any pushes
// queued by a prior cycle, Flush sends this cycle's coalesced pushes.
// *notifydispatch.PushCoalescer satisfies it structurally.
type pushFlusher interface {
	Reset()
	Flush(ctx context.Context) error
}

// Seeder runs one dev-seed cycle: read dev's watch zones, pull the
// most-recently-changed prod applications that fall within any of them, and
// feed each through the real ingest pipeline.
type Seeder struct {
	zones    zoneLister
	prodApps prodReader
	ingester *polling.Ingester
	push     pushFlusher
	limit    int
	logger   *slog.Logger
}

// NewSeeder wires a Seeder.
func NewSeeder(zones zoneLister, prodApps prodReader, ingester *polling.Ingester, push pushFlusher, limit int, logger *slog.Logger) *Seeder {
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
	watchZones, err := s.zones.All(ctx)
	if err != nil {
		return 0, err
	}
	if len(watchZones) == 0 {
		s.logger.InfoContext(ctx, "dev-seed: no watch zones, skipping cycle")
		return 0, nil
	}

	geometries := make([]applications.ZoneGeometry, len(watchZones))
	for i, z := range watchZones {
		geometries[i] = zoneGeometryOf(z)
	}

	apps, err := s.prodApps.RecentNearZones(ctx, geometries, s.limit)
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

// zoneGeometryOf maps a watch zone's shape into applications.ZoneGeometry.
// applications cannot import watchzones (which already imports applications,
// see applications.Coordinate's doc), so this mapping lives on the devseed
// side, which imports both. A custom-shape zone (non-empty Boundary) carries
// its own polygon ring; a circle zone carries its centre and radius with an
// empty Boundary — mirroring watchzones.WatchZone.IsCustomShape.
func zoneGeometryOf(z watchzones.WatchZone) applications.ZoneGeometry {
	g := applications.ZoneGeometry{
		Latitude:     z.Latitude,
		Longitude:    z.Longitude,
		RadiusMetres: z.RadiusMetres,
	}
	if z.IsCustomShape() {
		g.Boundary = make([]applications.Coordinate, len(z.Boundary))
		for i, v := range z.Boundary {
			g.Boundary[i] = applications.Coordinate{Longitude: v.Longitude, Latitude: v.Latitude}
		}
	}
	return g
}
