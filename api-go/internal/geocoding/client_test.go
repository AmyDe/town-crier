package geocoding

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
	client := NewClient(srv.URL, srv.Client())

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
	client := NewClient(srv.URL, srv.Client())

	if _, _, err := client.Geocode(context.Background(), "SW1A 1AA"); err != nil {
		t.Fatalf("Geocode: %v", err)
	}
	// The space is percent-escaped as a single path segment, mirroring .NET's
	// Uri.EscapeDataString.
	if want := "/postcodes/SW1A%201AA"; fake.requestedPath != want {
		t.Errorf("requested path = %q, want %q", fake.requestedPath, want)
	}
}

func TestClient_Geocode_NotFoundOnNon2xx(t *testing.T) {
	t.Parallel()

	fake := &postcodesIoServer{status: http.StatusNotFound, body: `{"status":404,"error":"Postcode not found"}`}
	srv := newPostcodesIoServer(t, fake)
	client := NewClient(srv.URL, srv.Client())

	// A non-2xx response means not found (a 404 for the caller), not an error —
	// mirroring .NET's null return on !IsSuccessStatusCode.
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

	// A 2xx wrapper whose envelope status is not 200 is still "not found",
	// matching .NET's body.Status != 200 short-circuit.
	fake := &postcodesIoServer{body: `{"status":404,"result":null}`}
	srv := newPostcodesIoServer(t, fake)
	client := NewClient(srv.URL, srv.Client())

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

	// status 200 but a null result is "not found", matching .NET's Result is null.
	fake := &postcodesIoServer{body: `{"status":200,"result":null}`}
	srv := newPostcodesIoServer(t, fake)
	client := NewClient(srv.URL, srv.Client())

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
	// caller), mirroring .NET's propagated HttpRequestException. Build a server,
	// capture its URL, then close it so the connection is refused.
	fake := &postcodesIoServer{body: `{}`}
	srv := newPostcodesIoServer(t, fake)
	client := NewClient(srv.URL, srv.Client())
	srv.Close()

	if _, _, err := client.Geocode(context.Background(), "SW1A 1AA"); err == nil {
		t.Error("Geocode on dead upstream: want error, got nil")
	}
}
