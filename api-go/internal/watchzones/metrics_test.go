package watchzones

import (
	"context"
	"log/slog"
	"net/http"
	"testing"
)

// fakeZoneMetrics records the watch-zone lifecycle calls the handlers make. It
// satisfies the watchzones consumer-side MetricsRecorder.
type fakeZoneMetrics struct {
	created int
	updated int
	deleted int
}

func (f *fakeZoneMetrics) WatchZoneCreated(context.Context) { f.created++ }
func (f *fakeZoneMetrics) WatchZoneUpdated(context.Context) { f.updated++ }
func (f *fakeZoneMetrics) WatchZoneDeleted(context.Context) { f.deleted++ }

func testMuxWithMetrics(t *testing.T, store zoneStore, rec MetricsRecorder) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	Routes(mux, store, slog.New(slog.DiscardHandler), WithMetricsRecorder(rec))
	return mux
}

func TestHandler_Patch_RecordsWatchZoneUpdated(t *testing.T) {
	t.Parallel()
	z := testZone(t)
	store := &fakeZoneStore{zones: []WatchZone{z}}
	rec := &fakeZoneMetrics{}
	mux := testMuxWithMetrics(t, store, rec)

	resp := doReq(t, mux, http.MethodPatch, "/v1/me/watch-zones/"+z.ID, `{"name":"Office"}`)
	if resp.Code != http.StatusOK {
		t.Fatalf("status: got %d (body %s)", resp.Code, resp.Body)
	}
	if rec.updated != 1 {
		t.Errorf("WatchZoneUpdated = %d, want 1", rec.updated)
	}
	if rec.created != 0 || rec.deleted != 0 {
		t.Errorf("unexpected create/delete counters: created=%d deleted=%d", rec.created, rec.deleted)
	}
}

func TestHandler_Patch_DoesNotRecordOnNotFound(t *testing.T) {
	t.Parallel()
	store := &fakeZoneStore{}
	rec := &fakeZoneMetrics{}
	mux := testMuxWithMetrics(t, store, rec)

	resp := doReq(t, mux, http.MethodPatch, "/v1/me/watch-zones/missing", `{"name":"Office"}`)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", resp.Code)
	}
	if rec.updated != 0 {
		t.Errorf("WatchZoneUpdated must not fire on 404, got %d", rec.updated)
	}
}

func TestHandler_Delete_RecordsWatchZoneDeleted(t *testing.T) {
	t.Parallel()
	z := testZone(t)
	store := &fakeZoneStore{zones: []WatchZone{z}}
	rec := &fakeZoneMetrics{}
	mux := testMuxWithMetrics(t, store, rec)

	resp := doReq(t, mux, http.MethodDelete, "/v1/me/watch-zones/"+z.ID, "")
	if resp.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want 204", resp.Code)
	}
	if rec.deleted != 1 {
		t.Errorf("WatchZoneDeleted = %d, want 1", rec.deleted)
	}
}

func TestHandler_Delete_DoesNotRecordOnNotFound(t *testing.T) {
	t.Parallel()
	store := &fakeZoneStore{}
	rec := &fakeZoneMetrics{}
	mux := testMuxWithMetrics(t, store, rec)

	resp := doReq(t, mux, http.MethodDelete, "/v1/me/watch-zones/missing", "")
	if resp.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", resp.Code)
	}
	if rec.deleted != 0 {
		t.Errorf("WatchZoneDeleted must not fire on 404, got %d", rec.deleted)
	}
}
