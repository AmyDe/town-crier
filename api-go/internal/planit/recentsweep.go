// GH#1134 / ADR 0047: Lane E, the looping recent-window start_date sweep that
// backstops Lanes A/B. This file adds the one query shape Lane E needs: a
// national (no-auth), two-sided date-windowed sweep sorted on start_date, with
// a LIGHT projection — just enough to detect a divergence against Postgres —
// so a happy-path lap that should be almost entirely no-ops serialises roughly
// fifteen times fewer bytes than the full ingest set would. A genuine
// divergence is hydrated to the full record via the existing FetchByUID
// (national.go), unchanged.
package planit

import (
	"context"
	"fmt"
	"time"
)

// recentSweepSelectFields is ADR 0047 Lane E's light national projection.
// start_date MUST be present: it is the sort field, and PlanIt 400s when a sort
// field is absent from select (see ingestSelectFields on last_different).
// last_different is deliberately EXCLUDED: nothing in Lane E's divergence test
// (polling.inverseMaskDiffers, reused from Lane C) reads it, and a PlanIt
// re-index bumps it, so carrying it would only invite the churn it is designed
// to filter out. area_id is required on every row so a national query can build
// the correct authority scope for the existence check — PlanIt's uid is unique
// only within one authority.
var recentSweepSelectFields = []string{"uid", "area_id", "app_state", "decided_date", "start_date"}

// FetchRecentSweepPage fetches one page of Lane E's national, date-windowed
// recent-band sweep: no auth param, bounded both above (windowEnd, fixed for
// the window's lifetime) and below (windowStart, the window's trailing edge),
// sorted -start_date, the light recentSweepSelectFields projection, pg_sz=300,
// compress=on. Throttling, retry, and 429 handling are identical to every
// other fetch method (shared fetchPage tail).
func (c *Client) FetchRecentSweepPage(ctx context.Context, windowStart, windowEnd time.Time, startIndex int) (FetchPageResult, error) {
	target := c.baseURL + buildRecentSweepPath(windowStart, windowEnd, startIndex)
	return c.fetchPage(ctx, target, 0, startIndex, nationalPageSize)
}

// buildRecentSweepPath builds Lane E's national recent-window sweep query path:
// a two-sided bounded window (start_date AND end_date — the shape ADR 0044
// measured cheap, never the unbounded one-sided shape), sort=-start_date,
// pg_sz=300, index, the light select projection (which contains start_date,
// satisfying PlanIt's "sort field must be selected" rule), and compress=on. No
// auth param.
func buildRecentSweepPath(windowStart, windowEnd time.Time, startIndex int) string {
	return fmt.Sprintf(
		"/api/applics/json?start_date=%s&end_date=%s&sort=-start_date&pg_sz=%d&index=%d&select=%s&compress=on",
		windowStart.UTC().Format("2006-01-02"),
		windowEnd.UTC().Format("2006-01-02"),
		nationalPageSize,
		startIndex,
		selectParam(recentSweepSelectFields),
	)
}
