package shareevents

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeStore struct {
	days         []time.Time
	destinations []string
	err          error
}

func (f *fakeStore) Increment(_ context.Context, day time.Time, destination string) error {
	if f.err != nil {
		return f.err
	}
	f.days = append(f.days, day)
	f.destinations = append(f.destinations, destination)
	return nil
}

func post(t *testing.T, store Store, body string) *httptest.ResponseRecorder {
	t.Helper()
	now := func() time.Time { return time.Date(2026, 10, 30, 23, 30, 0, 0, time.FixedZone("x", -3*3600)) }
	mux := http.NewServeMux()
	Routes(mux, store, now, slog.New(slog.DiscardHandler))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/share-events", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestRecord_MapsDestinationToClosedSet(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ body, want string }{
		{`{"destination":"messages"}`, "messages"},
		{`{"destination":"mail"}`, "mail"},
		{`{"destination":"whatsapp"}`, "whatsapp"},
		{`{"destination":"facebook"}`, "facebook"},
		{`{"destination":"copy"}`, "copy"},
		{`{"destination":"other"}`, "other"},
		{`{"destination":"signal"}`, "other"},
		{`{"destination":"WhatsApp"}`, "other"},
		{`{"destination":""}`, "other"},
		{`{}`, "other"},
		{`{"destination":"mail","code":"abc123","user":"u"}`, "mail"},
	} {
		t.Run(tc.body, func(t *testing.T) {
			t.Parallel()
			store := &fakeStore{}
			rec := post(t, store, tc.body)
			if rec.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want 204", rec.Code)
			}
			if len(store.destinations) != 1 || store.destinations[0] != tc.want {
				t.Errorf("destinations = %v, want [%s]", store.destinations, tc.want)
			}
		})
	}
}

func TestRecord_KeysTheTotalOnTheUTCDay(t *testing.T) {
	t.Parallel()
	store := &fakeStore{}
	post(t, store, `{"destination":"mail"}`)
	want := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	if len(store.days) != 1 || !store.days[0].Equal(want) {
		t.Errorf("days = %v, want %v", store.days, want)
	}
}

func TestRecord_RejectsBadBodies(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"not json":   "nope",
		"wrong type": `{"destination":5}`,
		"oversized":  `{"destination":"` + strings.Repeat("x", maxBodyBytes) + `"}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			store := &fakeStore{}
			rec := post(t, store, body)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
			if len(store.destinations) != 0 {
				t.Errorf("destinations = %v, want none", store.destinations)
			}
		})
	}
}

func TestRecord_StoreError_Returns500(t *testing.T) {
	t.Parallel()
	rec := post(t, &fakeStore{err: errors.New("boom")}, `{"destination":"mail"}`)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}
