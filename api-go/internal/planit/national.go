// ADR 0041 / GH#962: the churn-masked national delta poll. This file adds the
// query shapes the old per-authority drain (client.go) never needed: a
// national (no-auth) query masked by start_date or decided_start so the
// re-index churn that dominates the raw last_different axis is filtered out
// upstream, a select projection (mandatory on every one of these requests —
// PlanIt's other_fields is ~60% of a record's bytes and nothing here reads it
// back), and compress=on. FetchApplicationsPage (client.go) is unchanged and
// untouched by this file.
package planit

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// nationalPageSize is the pg_sz sent on every Lane A/B/C request (ADR 0041):
// PlanIt's own documented default, which its docs explicitly ask callers not
// to raise ("please don't try to get all the data in one request by setting
// this as your default page size"). Deliberately NOT Options.PageSize (which
// still governs the legacy per-authority FetchApplicationsPage drain,
// unwired but left compiling) — pg_sz=300 is a fixed safety rule for the new
// lanes, not an operator-tunable dial that could be raised by accident.
const nationalPageSize = 300

// uidPageSize is the pg_sz sent on a single-uid hydration lookup (Lane C):
// exactly one record is ever expected back.
const uidPageSize = 1

// ingestSelectFields lists every field the ingest pipeline consumes (ADR 0041
// / GH#962), in the exact order the build spec gives. last_different — the
// field every lane sorts on — MUST be present here: PlanIt 400s if a sort
// field is absent from select.
var ingestSelectFields = []string{
	"name", "uid", "area_name", "area_id", "address", "postcode", "description",
	"app_type", "app_state", "app_size", "start_date", "decided_date", "consulted_date",
	"location_x", "location_y", "url", "link", "last_different", "reference", "altid",
	"associated_id", "last_changed", "last_scraped", "scraper_name",
}

// reconciliationSelectFields is Lane C's light projection (ADR 0041): just
// enough to detect a straggler — a row whose PlanIt state has drifted from
// what Postgres holds — without paying for the full ~778 bytes/record ingest
// set.
var reconciliationSelectFields = []string{"uid", "app_state", "decided_date", "last_different"}

// MaskParam names the churn-mask query parameter ADR 0041 defines: Lane A
// masks on start_date (the council's own date, which a PlanIt re-index cannot
// move); Lane B masks on decided_start for the same reason, scoped to
// decisions.
type MaskParam string

// MaskStartDate and MaskDecidedStart are the only two valid MaskParam values.
const (
	MaskStartDate    MaskParam = "start_date"
	MaskDecidedStart MaskParam = "decided_start"
)

// NationalDeltaQuery configures one Lane A/B national delta-poll fetch: no
// auth param (national, no per-authority scope), a coarse date-granular
// different_start prefilter, a churn mask (field + cutoff), and the page's
// 0-based record offset. FetchNationalDeltaPage always sorts descending
// (-last_different), pages at pg_sz=300, and requests the ingest select
// projection with compress=on.
type NationalDeltaQuery struct {
	// DifferentStart is the coarse, date-granular different_start prefilter —
	// the calling lane's in-memory watermark's calendar date (or, on a lane's
	// first-ever run, the mask cutoff date itself, since there is no prior
	// watermark to prefilter on). This alone does NOT give exact delta
	// semantics (different_start is date-granular); the descending sort plus
	// the caller's in-memory timestamp watermark does that.
	DifferentStart time.Time
	// Mask is which date field narrows the query to genuinely new/changed
	// records: MaskStartDate (Lane A) or MaskDecidedStart (Lane B).
	Mask MaskParam
	// MaskCutoff is the mask's cutoff date (today minus the configured mask
	// window).
	MaskCutoff time.Time
	// StartIndex is this page's 0-based record offset (index=).
	StartIndex int
}

// FetchNationalDeltaPage fetches one page of PlanIt's national churn-masked
// delta query (ADR 0041 Lane A/B): no auth param, sort=-last_different,
// pg_sz=300, the full ingest select projection, and compress=on. Throttling,
// retry, and 429 handling are identical to FetchApplicationsPage.
func (c *Client) FetchNationalDeltaPage(ctx context.Context, q NationalDeltaQuery) (FetchPageResult, error) {
	target := c.baseURL + buildNationalDeltaPath(q)
	return c.fetchPage(ctx, target, 0, q.StartIndex, nationalPageSize)
}

// FetchReconciliationPage fetches one page of Lane C's light per-authority
// projection (ADR 0041): select=uid,app_state,decided_date,last_different,
// sorted newest-touched-first, pg_sz=300, scoped to authorityID.
func (c *Client) FetchReconciliationPage(ctx context.Context, authorityID, startIndex int) (FetchPageResult, error) {
	target := c.baseURL + buildReconciliationPath(authorityID, startIndex)
	return c.fetchPage(ctx, target, authorityID, startIndex, nationalPageSize)
}

// FetchByUID hydrates one straggler Lane C flagged: a single-record fetch via
// PlanIt's id_match filter, with the full ingest select projection. Whether
// id_match accepts a comma-separated uid list is unproven (ADR 0041), so this
// deliberately fetches one uid at a time — the ADR's explicitly sanctioned
// fallback.
func (c *Client) FetchByUID(ctx context.Context, uid string) (FetchPageResult, error) {
	target := c.baseURL + buildUIDPath(uid)
	return c.fetchPage(ctx, target, 0, 0, uidPageSize)
}

// selectParam joins a select-field list into PlanIt's comma-separated query
// value.
func selectParam(fields []string) string {
	return strings.Join(fields, ",")
}

// buildNationalDeltaPath builds the Lane A/B national query path: no auth,
// different_start (the coarse prefilter), the mask param, sort=-last_different,
// pg_sz=300, index, the ingest select projection (which contains
// last_different, satisfying PlanIt's "sort field must be selected" rule), and
// compress=on.
func buildNationalDeltaPath(q NationalDeltaQuery) string {
	return fmt.Sprintf(
		"/api/applics/json?different_start=%s&%s=%s&sort=-last_different&pg_sz=%d&index=%d&select=%s&compress=on",
		q.DifferentStart.UTC().Format("2006-01-02"),
		q.Mask,
		q.MaskCutoff.UTC().Format("2006-01-02"),
		nationalPageSize,
		q.StartIndex,
		selectParam(ingestSelectFields),
	)
}

// buildReconciliationPath builds Lane C's light per-authority projection path.
func buildReconciliationPath(authorityID, startIndex int) string {
	return fmt.Sprintf(
		"/api/applics/json?auth=%d&sort=-last_different&pg_sz=%d&index=%d&select=%s&compress=on",
		authorityID, nationalPageSize, startIndex, selectParam(reconciliationSelectFields),
	)
}

// buildUIDPath builds Lane C's single-record hydration path.
func buildUIDPath(uid string) string {
	return fmt.Sprintf(
		"/api/applics/json?id_match=%s&pg_sz=%d&select=%s&compress=on",
		url.QueryEscape(uid), uidPageSize, selectParam(ingestSelectFields),
	)
}
