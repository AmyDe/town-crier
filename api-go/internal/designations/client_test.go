package designations

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mustNewClient calls NewClient and fatals the test on error. All test base
// URLs are valid httptest addresses so this should never fail in practice.
func mustNewClient(t *testing.T, baseURL string, httpClient *http.Client) *Client {
	t.Helper()
	c, err := NewClient(baseURL, httpClient)
	if err != nil {
		t.Fatalf("NewClient(%q): %v", baseURL, err)
	}
	return c
}

// TestClient_DoesNotFollowCrossHostRedirect asserts that the designations
// client refuses a 302 redirect whose target host differs from the configured
// base host. The redirected body (from the second server) must not be returned.
func TestClient_DoesNotFollowCrossHostRedirect(t *testing.T) {
	t.Parallel()

	// attackServer simulates a cross-host SSRF target; its body must never be
	// read by the client.
	attackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Return a valid entity response with a sentinel name that, if seen,
		// proves the redirect was followed.
		_, _ = w.Write([]byte(`{"entities":[{"dataset":"conservation-area","name":"SSRF-FOLLOWED","reference":"X1"}]}`))
	}))
	t.Cleanup(attackServer.Close)

	// configuredServer redirects cross-host to attackServer.
	configuredServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, attackServer.URL+r.URL.RequestURI(), http.StatusFound)
	}))
	t.Cleanup(configuredServer.Close)

	client := mustNewClient(t, configuredServer.URL, configuredServer.Client())

	got, err := client.Get(context.Background(), 51.5, -0.14)

	// The client must not follow the cross-host redirect. Either an error is
	// returned, or the context is empty (redirect response, not the attack body).
	// If the client followed the redirect the sentinel name would appear.
	if err == nil && got.IsWithinConservationArea && got.ConservationAreaName != nil && *got.ConservationAreaName == "SSRF-FOLLOWED" {
		t.Error("client followed cross-host redirect: got SSRF-FOLLOWED conservation area, want policy to block it")
	}
}

// govUkServer is a hand-written fake planning.data.gov.uk entity endpoint. It
// records the raw request URI and drives the status code and JSON body.
type govUkServer struct {
	requestedURI string
	status       int    // status for /api/v1/entity.json; 0 -> 200
	body         string // raw JSON body on a 2xx
}

func newGovUkServer(t *testing.T, s *govUkServer) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// RequestURI is the unmodified request target, so the escaping assertion
		// sees exactly what went on the wire.
		s.requestedURI = r.RequestURI
		if s.status != 0 {
			w.WriteHeader(s.status)
		}
		_, _ = w.Write([]byte(s.body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestClient_Get_MapsAllDatasets(t *testing.T) {
	t.Parallel()

	fake := &govUkServer{body: `{"entities":[
		{"dataset":"conservation-area","name":"Old Town CA","reference":"CA1"},
		{"dataset":"listed-building-outline","name":"The Hall","reference":"LB1","listed-building-grade":"II*"},
		{"dataset":"article-4-direction-area","name":"A4 Zone","reference":"A41"}
	]}`}
	srv := newGovUkServer(t, fake)
	client := mustNewClient(t, srv.URL, srv.Client())

	got, err := client.Get(context.Background(), 51.5, -0.14)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.IsWithinConservationArea || got.ConservationAreaName == nil || *got.ConservationAreaName != "Old Town CA" {
		t.Errorf("conservation = %+v", got)
	}
	if !got.IsWithinListedBuildingCurtilage || got.ListedBuildingGrade == nil || *got.ListedBuildingGrade != "II*" {
		t.Errorf("listed building = %+v", got)
	}
	if !got.IsWithinArticle4Area {
		t.Errorf("article4 = %+v", got)
	}
}

func TestClient_Get_EscapesPointLongitudeFirst(t *testing.T) {
	t.Parallel()

	fake := &govUkServer{status: http.StatusNotFound}
	srv := newGovUkServer(t, fake)
	client := mustNewClient(t, srv.URL, srv.Client())

	if _, err := client.Get(context.Background(), 55, 2); err != nil {
		t.Fatalf("Get: %v", err)
	}
	// POINT(longitude latitude) with .NET-style escaping: "(" -> %28, " " -> %20,
	// ")" -> %29; the dataset commas stay literal.
	want := "/api/v1/entity.json?geometry_intersects=POINT%282%2055%29&dataset=conservation-area,listed-building-outline,article-4-direction-area"
	if fake.requestedURI != want {
		t.Errorf("request URI =\n  %q\nwant\n  %q", fake.requestedURI, want)
	}
}

func TestClient_Get_NotFoundIsEmptyContext(t *testing.T) {
	t.Parallel()

	// 404 means the geometry intersects no entity — an empty context, not an error.
	fake := &govUkServer{status: http.StatusNotFound}
	srv := newGovUkServer(t, fake)
	client := mustNewClient(t, srv.URL, srv.Client())

	got, err := client.Get(context.Background(), 55, 2)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if (got != Context{}) {
		t.Errorf("got %+v, want empty context", got)
	}
}

func TestClient_Get_EmptyEntitiesIsEmptyContext(t *testing.T) {
	t.Parallel()

	fake := &govUkServer{body: `{"entities":[]}`}
	srv := newGovUkServer(t, fake)
	client := mustNewClient(t, srv.URL, srv.Client())

	got, err := client.Get(context.Background(), 55, 2)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if (got != Context{}) {
		t.Errorf("got %+v, want empty context", got)
	}
}

func TestClient_Get_ErrorOnServerError(t *testing.T) {
	t.Parallel()

	// A non-404 error status is an error (the handler maps it to the empty
	// context), mirroring .NET's EnsureSuccessStatusCode throw.
	fake := &govUkServer{status: http.StatusInternalServerError, body: `{}`}
	srv := newGovUkServer(t, fake)
	client := mustNewClient(t, srv.URL, srv.Client())

	if _, err := client.Get(context.Background(), 55, 2); err == nil {
		t.Error("Get on 500: want error, got nil")
	}
}

func TestClient_Get_ErrorOnTransportFailure(t *testing.T) {
	t.Parallel()

	fake := &govUkServer{body: `{}`}
	srv := newGovUkServer(t, fake)
	client := mustNewClient(t, srv.URL, srv.Client())
	srv.Close()

	if _, err := client.Get(context.Background(), 55, 2); err == nil {
		t.Error("Get on dead upstream: want error, got nil")
	}
}

func TestEscapeDataString_MatchesUriEscapeDataString(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"POINT(2 55)":       "POINT%282%2055%29",
		"POINT(-0.14 51.5)": "POINT%28-0.14%2051.5%29",
		"abc-_.~":           "abc-_.~",
	}
	for in, want := range tests {
		if got := escapeDataString(in); got != want {
			t.Errorf("escapeDataString(%q) = %q, want %q", in, got, want)
		}
	}
}
