package tc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
)

// runBackfillWatchZoneLocation implements `tc backfill-watchzone-location`: a
// guarded, idempotent one-shot that POSTs to /v1/admin/watchzones/backfill-location
// (no body) and prints the reconciled counts. The endpoint rewrites every legacy
// WatchZone document with a derived GeoJSON location; re-running it is safe.
func runBackfillWatchZoneLocation(ctx context.Context, client *Client, env Env, _ *ParsedArgs) int {
	resp, err := client.Post(ctx, "/v1/admin/watchzones/backfill-location", nil)
	if err != nil {
		fmt.Fprintf(env.Err, "API error: %s\n", err)
		return exitRuntime
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
		fmt.Fprintf(env.Err, "API error (%d): %s\n", resp.StatusCode, string(body))
		return exitRuntime
	}

	var result backfillWatchZoneLocationResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxRespBytes)).Decode(&result); err != nil {
		fmt.Fprintf(env.Err, "API error: %s\n", err)
		return exitRuntime
	}

	fmt.Fprintf(env.Out, "Backfilled location on %d of %d watch zones (%d already had it)\n",
		result.Backfilled, result.Total, result.AlreadyHad)
	return exitOK
}
