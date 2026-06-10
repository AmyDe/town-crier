package legal

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	Routes(mux, slog.New(slog.DiscardHandler))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestRoutes_LegalDocument(t *testing.T) {
	t.Parallel()

	srv := newServer(t)

	tests := []struct {
		name             string
		documentType     string
		wantStatus       int
		wantContentType  string
		wantDocumentType string // value of the "documentType" field on 200
		wantTitle        string
	}{
		{"privacy lowercase", "privacy", http.StatusOK, "application/json; charset=utf-8", "privacy", "Privacy Policy"},
		// .NET upper-cases the lookup key (ToUpperInvariant) so case does not
		// matter on the route value; the body still reflects the file content.
		{"privacy uppercase", "PRIVACY", http.StatusOK, "application/json; charset=utf-8", "privacy", "Privacy Policy"},
		{"terms lowercase", "terms", http.StatusOK, "application/json; charset=utf-8", "terms", "Terms of Service"},
		{"terms mixed case", "Terms", http.StatusOK, "application/json; charset=utf-8", "terms", "Terms of Service"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/v1/legal/"+tc.documentType, nil)
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			resp, err := srv.Client().Do(req)
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status: got %d, want %d", resp.StatusCode, tc.wantStatus)
			}
			if got := resp.Header.Get("Content-Type"); got != tc.wantContentType {
				t.Errorf("content-type: got %q, want %q", got, tc.wantContentType)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}

			var doc struct {
				DocumentType string `json:"documentType"`
				Title        string `json:"title"`
				LastUpdated  string `json:"lastUpdated"`
				Sections     []struct {
					Heading string `json:"heading"`
					Body    string `json:"body"`
				} `json:"sections"`
			}
			if err := json.Unmarshal(body, &doc); err != nil {
				t.Fatalf("unmarshal body %q: %v", body, err)
			}
			if doc.DocumentType != tc.wantDocumentType {
				t.Errorf("documentType: got %q, want %q", doc.DocumentType, tc.wantDocumentType)
			}
			if doc.Title != tc.wantTitle {
				t.Errorf("title: got %q, want %q", doc.Title, tc.wantTitle)
			}
			if len(doc.Sections) == 0 {
				t.Errorf("sections: got 0, want > 0")
			}
		})
	}
}

func TestRoutes_LegalDocument_CompactCamelCaseWire(t *testing.T) {
	t.Parallel()

	srv := newServer(t)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/v1/legal/privacy", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	// .NET emits compact JSON (Results.Ok), not the pretty-printed file. Parity
	// requires the same: the body must start with the camelCase field in
	// declaration order and contain no indentation whitespace.
	const wantPrefix = `{"documentType":"privacy","title":"Privacy Policy","lastUpdated":`
	if got := string(body); len(got) < len(wantPrefix) || got[:len(wantPrefix)] != wantPrefix {
		t.Errorf("body prefix: got %.80q, want prefix %q", got, wantPrefix)
	}
	if contains(body, '\n') || contains(body, '\t') {
		t.Errorf("body must be compact (no newlines/tabs): got %.80q", body)
	}
}

func contains(b []byte, c byte) bool {
	for _, x := range b {
		if x == c {
			return true
		}
	}
	return false
}

func TestRoutes_LegalDocument_UnknownTypeReturns404(t *testing.T) {
	t.Parallel()

	srv := newServer(t)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/v1/legal/unknown", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	// Iteration 1 has no error-body backfill middleware yet (that lands in
	// iteration 2). The handler returns a bodyless 404 like .NET's
	// Results.NotFound() before the middleware adds the PascalCase backfill.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if len(body) != 0 {
		t.Errorf("body: got %q, want empty (backfill added by middleware in iteration 2)", body)
	}
}
