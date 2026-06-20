package geocoding

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

// TestClient_DoesNotFollowCrossHostRedirect asserts that the geocoding client
// refuses a 302 redirect whose target host differs from the configured base
// host. The redirected body (from the second server) must not be returned; the
// client must error or surface the redirect response, not the cross-host body.
func TestClient_DoesNotFollowCrossHostRedirect(t *testing.T) {
	t.Parallel()

	// attackServer simulates a cross-host SSRF target; its body must never be
	// read by the client.
	attackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":200,"result":{"postcode":"SW1A 1AA","latitude":99.0,"longitude":99.0}}`))
	}))
	t.Cleanup(attackServer.Close)

	// configuredServer redirects cross-host to attackServer.
	configuredServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, attackServer.URL+r.URL.Path, http.StatusFound)
	}))
	t.Cleanup(configuredServer.Close)

	client := mustNewClient(t, configuredServer.URL, configuredServer.Client())

	coords, found, err := client.Geocode(context.Background(), "SW1A 1AA")

	// The client must not follow the cross-host redirect. Either an error is
	// returned, or found is false (redirect response, not the attack body).
	// The attack server returns latitude=99.0; if the client followed the
	// redirect we'd see that value — a definitive signal of a policy failure.
	if err == nil && found && coords.Latitude == 99.0 {
		t.Errorf("client followed cross-host redirect: got coords %+v, want policy to block it", coords)
	}
}

// postcodesIoServer is a hand-written fake postcodes.io. It records the requested
// path and lets a test drive the status code and raw JSON body the lookup
// endpoint returns.
type postcodesIoServer struct {
	requestedPath string
	status        int    // status returned by /postcodes/{postcode}; 0 -> 200
	body          string // raw JSON body returned on a 2xx
}

func newPostcodesIoServer(t *testing.T, s *postcodesIoServer) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// EscapedPath preserves the on-the-wire %20 so the escaping assertion holds.
		s.requestedPath = r.URL.EscapedPath()
		if s.status != 0 {
			w.WriteHeader(s.status)
		}
		_, _ = w.Write([]byte(s.body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestClient_Geocode_ReturnsCoordinates(t *testing.T) {
	t.Parallel()

	fake := &postcodesIoServer{
		body: `{"status":200,"result":{"postcode":"SW1A 1AA","latitude":51.501009,"longitude":-0.141588,"admin_district":"Westminster"}}`,
	}
	srv := newPostcodesIoServer(t, fake)
	client := mustNewClient(t, srv.URL, srv.Client())

	coords, found, err := client.Geocode(context.Background(), "SW1A 1AA")
	if err != nil {
		t.Fatalf("Geocode: %v", err)
	}
	if !found {
		t.Fatal("found = false, want true")
	}
	if coords.Latitude != 51.501009 || coords.Longitude != -0.141588 {
		t.Errorf("coords = %+v, want {51.501009 -0.141588}", coords)
	}
}

func TestClient_Geocode_EscapesPostcodeInPath(t *testing.T) {
	t.Parallel()

	fake := &postcodesIoServer{body: `{"status":200,"result":{"latitude":1,"longitude":2}}`}
	srv := newPostcodesIoServer(t, fake)
	client := mustNewClient(t, srv.URL, srv.Client())

	if _, _, err := client.Geocode(context.Background(), "SW1A 1AA"); err != nil {
		t.Fatalf("Geocode: %v", err)
	}
	// The space is percent-escaped as a single path segment (url.PathEscape).
	if want := "/postcodes/SW1A%201AA"; fake.requestedPath != want {
		t.Errorf("requested path = %q, want %q", fake.requestedPath, want)
	}
}

func TestClient_Geocode_NotFoundOnNon2xx(t *testing.T) {
	t.Parallel()

	fake := &postcodesIoServer{status: http.StatusNotFound, body: `{"status":404,"error":"Postcode not found"}`}
	srv := newPostcodesIoServer(t, fake)
	client := mustNewClient(t, srv.URL, srv.Client())

	// A non-2xx response means not found (a 404 for the caller), not an error.
	coords, found, err := client.Geocode(context.Background(), "ZZ1 1ZZ")
	if err != nil {
		t.Fatalf("Geocode: %v", err)
	}
	if found {
		t.Errorf("found = true, want false (got %+v)", coords)
	}
}

func TestClient_Geocode_NotFoundOnEnvelopeStatusNot200(t *testing.T) {
	t.Parallel()

	// A 2xx wrapper whose envelope status is not 200 is still "not found".
	fake := &postcodesIoServer{body: `{"status":404,"result":null}`}
	srv := newPostcodesIoServer(t, fake)
	client := mustNewClient(t, srv.URL, srv.Client())

	_, found, err := client.Geocode(context.Background(), "ZZ1 1ZZ")
	if err != nil {
		t.Fatalf("Geocode: %v", err)
	}
	if found {
		t.Error("found = true, want false")
	}
}

func TestClient_Geocode_NotFoundOnNilResult(t *testing.T) {
	t.Parallel()

	// status 200 but a null result is "not found".
	fake := &postcodesIoServer{body: `{"status":200,"result":null}`}
	srv := newPostcodesIoServer(t, fake)
	client := mustNewClient(t, srv.URL, srv.Client())

	_, found, err := client.Geocode(context.Background(), "ZZ1 1ZZ")
	if err != nil {
		t.Fatalf("Geocode: %v", err)
	}
	if found {
		t.Error("found = true, want false")
	}
}

func TestClient_Geocode_ErrorOnTransportFailure(t *testing.T) {
	t.Parallel()

	// A dead upstream is a transport failure: it returns an error (a 500 for the
	// caller). Build a server,
	// capture its URL, then close it so the connection is refused.
	fake := &postcodesIoServer{body: `{}`}
	srv := newPostcodesIoServer(t, fake)
	client := mustNewClient(t, srv.URL, srv.Client())
	srv.Close()

	if _, _, err := client.Geocode(context.Background(), "SW1A 1AA"); err == nil {
		t.Error("Geocode on dead upstream: want error, got nil")
	}
}
