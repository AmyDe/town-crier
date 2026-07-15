// GH#967 / ADR 0042: Lane D, the paced historical backfill lane. This file
// adds the one query shape the ADR 0041 lanes never needed: a national,
// date-WINDOWED (both start_date and end_date bounded) backward sweep sorted
// on start_date itself, rather than a delta on last_different. It reuses
// ingestSelectFields (national.go) verbatim so the sweep enriches every
// GH#935 field, and nationalPageSize (300) — the same fixed safety rule every
// ADR 0041 lane uses. No auth param: like Lane A/B/C's own national queries,
// this touches every authority's applications in the window at once.
package planit

import (
	"context"
	"fmt"
	"time"
)

// FetchBackfillPage fetches one page of Lane D's national, date-windowed
// backward sweep: no auth param, bounded both above (windowEnd, fixed for the
// window's lifetime) and below (windowStart, the window's trailing edge),
// sorted -start_date, the full ingest select projection (so every returned
// record can enrich the GH#935 fields via the standard Ingester), pg_sz=300,
// compress=on. Throttling, retry, and 429 handling are identical to every
// other fetch method (shared fetchPage tail).
func (c *Client) FetchBackfillPage(ctx context.Context, windowStart, windowEnd time.Time, startIndex int) (FetchPageResult, error) {
	target := c.baseURL + buildBackfillPath(windowStart, windowEnd, startIndex)
	return c.fetchPage(ctx, target, 0, startIndex, nationalPageSize)
}

// buildBackfillPath builds Lane D's national backward-sweep query path: a
// two-sided bounded window (start_date AND end_date — the shape ADR 0041
// measured at 11.7s, never the unbounded one-sided shape it measured at 45s/
// total:null), sort=-start_date (start_date is already in ingestSelectFields,
// satisfying PlanIt's "sort field must be selected" rule with no changes
// there), pg_sz=300, index, the full ingest select projection, and
// compress=on. No auth param.
func buildBackfillPath(windowStart, windowEnd time.Time, startIndex int) string {
	return fmt.Sprintf(
		"/api/applics/json?start_date=%s&end_date=%s&sort=-start_date&pg_sz=%d&index=%d&select=%s&compress=on",
		windowStart.UTC().Format("2006-01-02"),
		windowEnd.UTC().Format("2006-01-02"),
		nationalPageSize,
		startIndex,
		selectParam(ingestSelectFields),
	)
}
