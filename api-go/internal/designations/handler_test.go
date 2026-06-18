package designations

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeProvider is a hand-written designation provider double. It records the
// coordinates it was asked about and returns a canned context / error.
type fakeProvider struct {
	gotLatitude  float64
	gotLongitude float64
	context      Context
	err          error
}

func (f *fakeProvider) Get(_ context.Context, latitude, longitude float64) (Context, error) {
	f.gotLatitude = latitude
	f.gotLongitude = longitude
	return f.context, f.err
}

func newDesignationsRequest(t *testing.T, p provider, query string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	Routes(mux, p, slog.New(slog.DiscardHandler))
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/designations"+query, nil)
	mux.ServeHTTP(rec, req)
	return rec
}

func strPtr(s string) *string { return &s }

func TestHandler_Designations_ReturnsContext(t *testing.T) {
	t.Parallel()

	fake := &fakeProvider{context: Context{
		IsWithinConservationArea:        true,
		ConservationAreaName:            strPtr("Old Town CA"),
		IsWithinListedBuildingCurtilage: true,
		ListedBuildingGrade:             strPtr("II*"),
		IsWithinArticle4Area:            true,
	}}
	rec := newDesignationsRequest(t, fake, "?latitude=51.5&longitude=-0.14")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	want := `{"isWithinConservationArea":true,"conservationAreaName":"Old Town CA","isWithinListedBuildingCurtilage":true,"listedBuildingGrade":"II*","isWithinArticle4Area":true}`
	if got := rec.Body.String(); got != want {
		t.Errorf("body =\n  %s\nwant\n  %s", got, want)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("content-type = %q", ct)
	}
}

func TestHandler_Designations_PassesParsedCoordinates(t *testing.T) {
	t.Parallel()

	fake := &fakeProvider{}
	newDesignationsRequest(t, fake, "?latitude=55&longitude=2")

	if fake.gotLatitude != 55 || fake.gotLongitude != 2 {
		t.Errorf("provider saw lat=%v lng=%v, want 55 / 2", fake.gotLatitude, fake.gotLongitude)
	}
}

func TestHandler_Designations_EmptyContextSerializesNulls(t *testing.T) {
	t.Parallel()

	fake := &fakeProvider{} // zero Context -> all false / null
	rec := newDesignationsRequest(t, fake, "?latitude=55&longitude=2")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	want := `{"isWithinConservationArea":false,"conservationAreaName":null,"isWithinListedBuildingCurtilage":false,"listedBuildingGrade":null,"isWithinArticle4Area":false}`
	if got := rec.Body.String(); got != want {
		t.Errorf("body =\n  %s\nwant\n  %s", got, want)
	}
}

func TestHandler_Designations_BadRequestOnMissingCoordinates(t *testing.T) {
	t.Parallel()

	for _, query := range []string{"", "?latitude=51.5", "?longitude=-0.14", "?latitude=abc&longitude=2"} {
		fake := &fakeProvider{}
		rec := newDesignationsRequest(t, fake, query)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("query %q: status = %d, want 400", query, rec.Code)
		}
		if rec.Body.Len() != 0 {
			t.Errorf("query %q: 400 body = %q, want empty (backfilled downstream)", query, rec.Body.String())
		}
	}
}

func TestHandler_Designations_ProviderErrorDegradesToEmpty(t *testing.T) {
	t.Parallel()

	// A provider failure must not fail the request: the handler answers 200 with
	// the empty context, mirroring .NET's catch(HttpRequestException) -> None.
	fake := &fakeProvider{err: errors.New("gov.uk down")}
	rec := newDesignationsRequest(t, fake, "?latitude=55&longitude=2")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	want := `{"isWithinConservationArea":false,"conservationAreaName":null,"isWithinListedBuildingCurtilage":false,"listedBuildingGrade":null,"isWithinArticle4Area":false}`
	if got := rec.Body.String(); got != want {
		t.Errorf("body =\n  %s\nwant\n  %s", got, want)
	}
}

func TestClient_SatisfiesProviderInterface(t *testing.T) {
	t.Parallel()

	client := mustNewClient(t, "https://www.planning.data.gov.uk", http.DefaultClient)
	var _ provider = client
}
