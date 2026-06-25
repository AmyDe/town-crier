package geocoding

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// reverseServer is a hand-written fake postcodes.io reverse endpoint that
// records the requested path+query and returns a driven status/body.
type reverseServer struct {
	requestedPath  string
	requestedQuery string
	status         int
	body           string
}

func newReverseServer(t *testing.T, s *reverseServer) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.requestedPath = r.URL.Path
		s.requestedQuery = r.URL.RawQuery
		if s.status != 0 {
			w.WriteHeader(s.status)
		}
		_, _ = w.Write([]byte(s.body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestClient_ResolveAuthority_MapsAdminDistrictToAuthorityID(t *testing.T) {
	t.Parallel()
	fake := &reverseServer{
		body: `{"status":200,"result":[{"postcode":"YO1 7HH","admin_district":"York"}]}`,
	}
	srv := newReverseServer(t, fake)
	client := mustNewClient(t, srv.URL, srv.Client())

	id, err := client.ResolveAuthority(context.Background(), 53.9590, -1.0815)
	if err != nil {
		t.Fatalf("ResolveAuthority: %v", err)
	}
	if id != 14 {
		t.Errorf("authority id: got %d, want 14 (York)", id)
	}
	if fake.requestedPath != "/postcodes" {
		t.Errorf("path: got %q, want /postcodes", fake.requestedPath)
	}
	if !strings.Contains(fake.requestedQuery, "lat=") || !strings.Contains(fake.requestedQuery, "lon=") {
		t.Errorf("query missing lat/lon: %q", fake.requestedQuery)
	}
}

func TestClient_ResolveAuthority_NoAdminDistrictIsUnresolved(t *testing.T) {
	t.Parallel()
	fake := &reverseServer{body: `{"status":200,"result":[]}`}
	srv := newReverseServer(t, fake)
	client := mustNewClient(t, srv.URL, srv.Client())

	_, err := client.ResolveAuthority(context.Background(), 0, 0)
	if !errors.Is(err, ErrAuthorityUnresolved) {
		t.Errorf("expected ErrAuthorityUnresolved, got %v", err)
	}
}

func TestClient_ResolveAuthority_UnmappedDistrictIsUnresolved(t *testing.T) {
	t.Parallel()
	fake := &reverseServer{
		body: `{"status":200,"result":[{"admin_district":"Atlantis"}]}`,
	}
	srv := newReverseServer(t, fake)
	client := mustNewClient(t, srv.URL, srv.Client())

	_, err := client.ResolveAuthority(context.Background(), 51.5, -0.1)
	if !errors.Is(err, ErrAuthorityUnresolved) {
		t.Errorf("expected ErrAuthorityUnresolved for unmapped district, got %v", err)
	}
}

func TestClient_ResolveAuthority_Non2xxIsUnresolved(t *testing.T) {
	t.Parallel()
	fake := &reverseServer{status: http.StatusInternalServerError, body: ""}
	srv := newReverseServer(t, fake)
	client := mustNewClient(t, srv.URL, srv.Client())

	_, err := client.ResolveAuthority(context.Background(), 51.5, -0.1)
	if !errors.Is(err, ErrAuthorityUnresolved) {
		t.Errorf("expected ErrAuthorityUnresolved on non-2xx, got %v", err)
	}
}

func TestAuthorityMapping_LoadedAndNonEmpty(t *testing.T) {
	t.Parallel()
	if len(authorityMapping) == 0 {
		t.Fatal("authority mapping failed to load")
	}
	if authorityMapping["Westminster"] != 326 {
		t.Errorf("Westminster mapping: got %d, want 326", authorityMapping["Westminster"])
	}
}

// TestAuthorityMapping_CombinedPlanningAuthorities guards tc-zuxq: PlanIt files
// planning applications for districts that share a Combined Planning Authority
// under the COMBINED area_id, not the standalone English District id. A watch
// zone resolved to the empty standalone partition returns zero applications, so
// these districts must map to their combined area. Verified against PlanIt: the
// standalone district areas hold no applications while the combined area holds
// them all (e.g. Worthing 251 empty, Adur and Worthing 449 populated).
func TestAuthorityMapping_CombinedPlanningAuthorities(t *testing.T) {
	t.Parallel()
	combined := map[string]int{
		"Adur":      449, // Adur and Worthing
		"Worthing":  449, // Adur and Worthing
		"Maidstone": 508, // Mid Kent
		"Swale":     508, // Mid Kent
	}
	for district, want := range combined {
		if got := authorityMapping[district]; got != want {
			t.Errorf("%s: got authority %d, want %d (combined planning authority)", district, got, want)
		}
	}
	// Tunbridge Wells shares the Mid Kent support service but still files under
	// its own area_id (verified: 1073 applications), so it must NOT be remapped
	// to the combined authority.
	if got := authorityMapping["Tunbridge Wells"]; got != 149 {
		t.Errorf("Tunbridge Wells: got %d, want 149 (files standalone, not under Mid Kent 508)", got)
	}
}
