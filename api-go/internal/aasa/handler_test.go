package aasa

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// aasaDocument mirrors the on-the-wire AASA shape so the test parses the served
// body back and asserts the exact contract (appID + /a/* component) rather than
// matching a byte string.
type aasaDocument struct {
	Applinks struct {
		Details []struct {
			AppIDs     []string `json:"appIDs"`
			Components []struct {
				Path string `json:"/"`
			} `json:"components"`
		} `json:"details"`
	} `json:"applinks"`
}

func TestAASA_ServesJSONDocument(t *testing.T) {
	t.Parallel()

	const appID = "4574VQ7N2X.uk.towncrierapp.mobile"
	mux := http.NewServeMux()
	Routes(mux, appID, slog.New(slog.DiscardHandler))

	// Apple fetches the extensionless well-known path; the content type, not the
	// path, identifies the document.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/.well-known/apple-app-site-association", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	// Apple's daemon requires exactly application/json.
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	var doc aasaDocument
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("body is not valid JSON: %v (%q)", err, rec.Body.String())
	}
	if len(doc.Applinks.Details) != 1 {
		t.Fatalf("applinks.details = %d, want 1", len(doc.Applinks.Details))
	}
	d := doc.Applinks.Details[0]
	if len(d.AppIDs) != 1 || d.AppIDs[0] != appID {
		t.Errorf("appIDs = %v, want [%q]", d.AppIDs, appID)
	}
	if len(d.Components) != 1 || d.Components[0].Path != "/a/*" {
		t.Errorf("components = %+v, want one entry with / == /a/*", d.Components)
	}
}
