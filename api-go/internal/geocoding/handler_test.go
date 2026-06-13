package geocoding

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeGeocoder is a hand-written geocoder double. It records the postcode it was
// asked to resolve and returns canned coordinates / found / error values.
type fakeGeocoder struct {
	gotPostcode string
	coords      Coordinates
	found       bool
	err         error
}

func (f *fakeGeocoder) Geocode(_ context.Context, postcode string) (Coordinates, bool, error) {
	f.gotPostcode = postcode
	return f.coords, f.found, f.err
}

func newGeocodeRequest(t *testing.T, g geocoder, rawPostcode string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	Routes(mux, g, slog.New(slog.DiscardHandler))
	rec := httptest.NewRecorder()
	// The path segment is pre-escaped so the mux decodes it back to rawPostcode,
	// matching how a real request arrives.
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/geocode/"+escapeSegment(rawPostcode), nil)
	mux.ServeHTTP(rec, req)
	return rec
}

// escapeSegment percent-escapes a space so the raw postcode survives as one path
// segment; the realistic inputs here contain only letters, digits and spaces.
func escapeSegment(s string) string {
	out := ""
	for _, c := range s {
		if c == ' ' {
			out += "%20"
			continue
		}
		out += string(c)
	}
	return out
}

func TestHandler_Geocode_ReturnsCoordinates(t *testing.T) {
	t.Parallel()

	fake := &fakeGeocoder{coords: Coordinates{Latitude: 51.501009, Longitude: -0.141588}, found: true}
	rec := newGeocodeRequest(t, fake, "SW1A 1AA")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != `{"coordinates":{"latitude":51.501009,"longitude":-0.141588}}` {
		t.Errorf("body = %s", got)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("content-type = %q", ct)
	}
}

func TestHandler_Geocode_NormalisesBeforeLookup(t *testing.T) {
	t.Parallel()

	fake := &fakeGeocoder{coords: Coordinates{Latitude: 1, Longitude: 2}, found: true}
	newGeocodeRequest(t, fake, "sw1a 1aa")

	// The geocoder is handed the trimmed, upper-cased postcode, never the raw input.
	if fake.gotPostcode != "SW1A 1AA" {
		t.Errorf("geocoder saw %q, want normalised SW1A 1AA", fake.gotPostcode)
	}
}

func TestHandler_Geocode_BadRequestOnInvalidPostcode(t *testing.T) {
	t.Parallel()

	fake := &fakeGeocoder{}
	rec := newGeocodeRequest(t, fake, "NOTAPOSTCODE")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if got := rec.Body.String(); got != `{"error":"'NOTAPOSTCODE' is not a valid UK postcode. (Parameter 'raw')","message":null}` {
		t.Errorf("body = %s", got)
	}
	if fake.gotPostcode != "" {
		t.Errorf("geocoder was called with %q; a malformed postcode must not reach it", fake.gotPostcode)
	}
}

func TestHandler_Geocode_NotFoundWhenUnresolvable(t *testing.T) {
	t.Parallel()

	// A valid-format postcode the geocoder cannot resolve is a 404 carrying the
	// raw input in the message, mirroring .NET's InvalidOperationException path.
	fake := &fakeGeocoder{found: false}
	rec := newGeocodeRequest(t, fake, "ZZ1 1ZZ")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if got := rec.Body.String(); got != `{"error":"Postcode 'ZZ1 1ZZ' could not be geocoded.","message":null}` {
		t.Errorf("body = %s", got)
	}
}

func TestHandler_Geocode_ServerErrorOnTransportFailure(t *testing.T) {
	t.Parallel()

	// A geocoder transport failure is a bodyless 500 (the PascalCase envelope is
	// backfilled downstream by middleware.ErrorBody), matching .NET's propagated
	// HttpRequestException.
	fake := &fakeGeocoder{err: errors.New("upstream down")}
	rec := newGeocodeRequest(t, fake, "SW1A 1AA")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("500 body = %q, want empty (backfilled downstream)", rec.Body.String())
	}
}

func TestClient_SatisfiesGeocoderInterface(t *testing.T) {
	t.Parallel()

	var _ geocoder = NewClient("https://api.postcodes.io", http.DefaultClient)
}
