package watchzones

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/authorities"
)

type fakeZoneLister struct {
	zones []WatchZone
	err   error
}

func (f *fakeZoneLister) GetByUserID(_ context.Context, _ string) ([]WatchZone, error) {
	return f.zones, f.err
}

// fakeLookup resolves only the ids it was seeded with.
type fakeLookup map[int]authorities.Authority

func (f fakeLookup) ByID(id int) (authorities.Authority, bool) {
	a, ok := f[id]
	return a, ok
}

func authorityZone(t *testing.T, authorityID int) WatchZone {
	t.Helper()
	z, err := NewWatchZone("z"+time.Now().Format("150405.000000000"), "u", "Z", 51, -0.1, 100, authorityID, time.Now(), true, true)
	if err != nil {
		t.Fatalf("NewWatchZone: %v", err)
	}
	return z
}

func serveAuthorities(t *testing.T, zones zoneAuthorityLister, lookup authorityLookup) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	AuthoritiesRoutes(mux, zones, lookup, slog.New(slog.DiscardHandler))
	req := httptest.NewRequestWithContext(auth.WithSubject(context.Background(), "auth0|u"), http.MethodGet, "/v1/me/application-authorities", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestApplicationAuthorities_DistinctSortedWithCount(t *testing.T) {
	t.Parallel()
	lister := &fakeZoneLister{zones: []WatchZone{
		authorityZone(t, 471), authorityZone(t, 9), authorityZone(t, 471), // 471 duplicated
	}}
	lookup := fakeLookup{
		471: {ID: 471, Name: "City of London", AreaType: "city"},
		9:   {ID: 9, Name: "Aberdeen", AreaType: "council"},
	}

	rec := serveAuthorities(t, lister, lookup)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("content-type: got %q", ct)
	}
	var got struct {
		Authorities []authorityItem `json:"authorities"`
		Count       int             `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Count != 2 || len(got.Authorities) != 2 {
		t.Fatalf("expected 2 distinct authorities, got %+v", got)
	}
	// Sorted by name (Aberdeen before City of London), duplicate 471 collapsed.
	if got.Authorities[0].ID != 9 || got.Authorities[1].ID != 471 {
		t.Errorf("order: %+v", got.Authorities)
	}
}

func TestApplicationAuthorities_EmptyArrayWhenNoZones(t *testing.T) {
	t.Parallel()
	rec := serveAuthorities(t, &fakeZoneLister{}, fakeLookup{})
	if got := rec.Body.String(); got != `{"authorities":[],"count":0}` {
		t.Errorf("empty body: got %s", got)
	}
}

func TestApplicationAuthorities_SkipsUnknownAuthority(t *testing.T) {
	t.Parallel()
	lister := &fakeZoneLister{zones: []WatchZone{authorityZone(t, 471), authorityZone(t, 99999)}}
	lookup := fakeLookup{471: {ID: 471, Name: "City of London", AreaType: "city"}}

	rec := serveAuthorities(t, lister, lookup)
	var got struct {
		Authorities []authorityItem `json:"authorities"`
		Count       int             `json:"count"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Count != 1 || got.Authorities[0].ID != 471 {
		t.Errorf("unknown authority not skipped: %+v", got)
	}
}
