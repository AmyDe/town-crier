package seocatalog

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	assignBatchSize      = 5000
	stateGazetteerHash   = "gazetteer_hash"
	stateAssignWatermark = "assign_watermark"
	upsertTownsSQL       = `INSERT INTO seo_towns (authority_code, slug, name, population, location)
SELECT code, slug, name, population, ST_SetSRID(ST_MakePoint(lng, lat), 4326)::geography
FROM unnest($1::text[], $2::text[], $3::text[], $4::int[], $5::float8[], $6::float8[]) AS t(code, slug, name, population, lat, lng)
ON CONFLICT (authority_code, slug) DO UPDATE
SET name = EXCLUDED.name, population = EXCLUDED.population, location = EXCLUDED.location`
	deleteStaleTownsSQL = `DELETE FROM seo_towns t
WHERE NOT EXISTS (
  SELECT 1 FROM unnest($1::text[], $2::text[]) AS g(code, slug)
  WHERE g.code = t.authority_code AND g.slug = t.slug)`
	batchBoundsSQL = `SELECT count(*), max(last_different) FROM (
  SELECT last_different FROM applications WHERE last_different > $1
  ORDER BY last_different, authority_code, planit_name LIMIT $2) b`
	removeAssignmentsSQL = `DELETE FROM seo_town_assignments s
USING applications a
WHERE a.authority_code = s.authority_code AND a.planit_name = s.planit_name
  AND a.last_different > $1 AND a.last_different <= $2
  AND (a.location IS NULL OR NOT EXISTS (
    SELECT 1 FROM seo_towns t
    WHERE t.authority_code = a.authority_code AND ST_DWithin(a.location, t.location, $3)))`
	upsertAssignmentsSQL = `INSERT INTO seo_town_assignments (authority_code, planit_name, town_slug, last_different)
SELECT a.authority_code, a.planit_name, t.slug, a.last_different
FROM applications a
CROSS JOIN LATERAL (
  SELECT slug FROM seo_towns t
  WHERE t.authority_code = a.authority_code AND ST_DWithin(a.location, t.location, $3)
  ORDER BY a.location <-> t.location, t.slug LIMIT 1) t
WHERE a.last_different > $1 AND a.last_different <= $2
ON CONFLICT (authority_code, planit_name) DO UPDATE
SET town_slug = EXCLUDED.town_slug, last_different = EXCLUDED.last_different`
	setStateSQL = `INSERT INTO seo_state (key, value) VALUES ($1, $2)
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`
)

// AssignSummary reports one Assign run.
type AssignSummary struct {
	Batches   int
	Processed int
	Reset     bool
}

// Assign syncs seo_towns from the gazetteer, then assigns every application
// changed since the stored watermark to its nearest town within
// AssignmentRadiusMetres in the same authority (exact ties go to the lower town
// slug), removing the assignment of one with no location or no town in range. A
// changed gazetteer clears all assignments and the watermark first.
func (s *Store) Assign(ctx context.Context) (AssignSummary, error) {
	var sum AssignSummary
	reset, err := s.syncGazetteer(ctx)
	if err != nil {
		return sum, err
	}
	sum.Reset = reset

	for {
		n, err := s.assignBatch(ctx)
		if err != nil {
			return sum, err
		}
		if n == 0 {
			return sum, nil
		}
		sum.Batches++
		sum.Processed += n
	}
}

func (s *Store) syncGazetteer(ctx context.Context) (reset bool, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin gazetteer sync: %w", err)
	}
	defer rollback(ctx, tx)

	if err = syncTownsTx(ctx, tx, s.gazetteer.Towns); err != nil {
		return false, err
	}

	var stored string
	switch qerr := tx.QueryRow(ctx, "SELECT value FROM seo_state WHERE key = $1", stateGazetteerHash).Scan(&stored); {
	case errors.Is(qerr, pgx.ErrNoRows):
	case qerr != nil:
		return false, fmt.Errorf("read gazetteer hash: %w", qerr)
	}
	if stored != s.gazetteer.Hash {
		reset = true
		if _, err = tx.Exec(ctx, "TRUNCATE seo_town_assignments"); err != nil {
			return false, fmt.Errorf("clear town assignments: %w", err)
		}
		if _, err = tx.Exec(ctx, "DELETE FROM seo_state WHERE key = $1", stateAssignWatermark); err != nil {
			return false, fmt.Errorf("reset assign watermark: %w", err)
		}
		if _, err = tx.Exec(ctx, setStateSQL, stateGazetteerHash, s.gazetteer.Hash); err != nil {
			return false, fmt.Errorf("store gazetteer hash: %w", err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit gazetteer sync: %w", err)
	}
	return reset, nil
}

func (s *Store) syncTowns(ctx context.Context) (err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin towns sync: %w", err)
	}
	defer rollback(ctx, tx)
	if err = syncTownsTx(ctx, tx, s.gazetteer.Towns); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit towns sync: %w", err)
	}
	return nil
}

func syncTownsTx(ctx context.Context, tx pgx.Tx, towns []Town) error {
	codes := make([]string, len(towns))
	slugs := make([]string, len(towns))
	names := make([]string, len(towns))
	populations := make([]int32, len(towns))
	lats := make([]float64, len(towns))
	lngs := make([]float64, len(towns))
	for i, t := range towns {
		codes[i] = strconv.Itoa(t.AuthorityID)
		slugs[i] = t.Slug
		names[i] = t.Name
		populations[i] = int32(t.Population)
		lats[i] = t.Lat
		lngs[i] = t.Lng
	}
	if _, err := tx.Exec(ctx, upsertTownsSQL, codes, slugs, names, populations, lats, lngs); err != nil {
		return fmt.Errorf("upsert seo_towns: %w", err)
	}
	if _, err := tx.Exec(ctx, deleteStaleTownsSQL, codes, slugs); err != nil {
		return fmt.Errorf("delete stale seo_towns: %w", err)
	}
	return nil
}

// assignBatch processes the next slice of changed applications in one
// transaction and returns how many it covered (0 when caught up). The slice is
// extended to every application sharing its newest last_different, so the
// advancing watermark never skips a tied row.
func (s *Store) assignBatch(ctx context.Context) (n int, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin assign batch: %w", err)
	}
	defer rollback(ctx, tx)

	watermark, err := readWatermark(ctx, tx)
	if err != nil {
		return 0, err
	}
	var upper *time.Time
	if err = tx.QueryRow(ctx, batchBoundsSQL, watermark, assignBatchSize).Scan(&n, &upper); err != nil {
		return 0, fmt.Errorf("find assign batch: %w", err)
	}
	if n == 0 || upper == nil {
		return 0, nil
	}
	radius := float64(AssignmentRadiusMetres)
	if _, err = tx.Exec(ctx, removeAssignmentsSQL, watermark, *upper, radius); err != nil {
		return 0, fmt.Errorf("remove stale assignments: %w", err)
	}
	if _, err = tx.Exec(ctx, upsertAssignmentsSQL, watermark, *upper, radius); err != nil {
		return 0, fmt.Errorf("assign applications to towns: %w", err)
	}
	if _, err = tx.Exec(ctx, setStateSQL, stateAssignWatermark, upper.UTC().Format(time.RFC3339Nano)); err != nil {
		return 0, fmt.Errorf("advance assign watermark: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit assign batch: %w", err)
	}
	return n, nil
}

func readWatermark(ctx context.Context, tx pgx.Tx) (time.Time, error) {
	var raw string
	err := tx.QueryRow(ctx, "SELECT value FROM seo_state WHERE key = $1", stateAssignWatermark).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Unix(0, 0).UTC(), nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("read assign watermark: %w", err)
	}
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse assign watermark %q: %w", raw, err)
	}
	return t, nil
}
