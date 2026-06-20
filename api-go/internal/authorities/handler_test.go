package authorities

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

// captured holds the parts of a response the assertions need, decoupled from
// the live *http.Response so the body is closed inside the helper.
type captured struct {
	status      int
	contentType string
	body        []byte
}

func do(t *testing.T, srv *httptest.Server, path string) captured {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+path, nil)
	if err != nil {
		t.Fatalf("new request %s: %v", path, err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("get %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body %s: %v", path, err)
	}
	return captured{
		status:      resp.StatusCode,
		contentType: resp.Header.Get("Content-Type"),
		body:        body,
	}
}

type listResponse struct {
	Authorities []struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		AreaType string `json:"areaType"`
	} `json:"authorities"`
	Total int `json:"total"`
}

func TestRoutes_AuthoritiesList(t *testing.T) {
	t.Parallel()

	srv := newServer(t)

	tests := []struct {
		name       string
		path       string
		wantTotal  int
		wantFirst  string // name of first item, "" to skip
		wantStatus int
	}{
		{"full list", "/v1/authorities", 485, "Aberdeen", http.StatusOK},
		{"search substring case-insensitive", "/v1/authorities?search=aberdeen", 2, "Aberdeen", http.StatusOK},
		{"search no match returns empty", "/v1/authorities?search=ZZZNOMATCH", 0, "", http.StatusOK},
		{"trailing slash matches list", "/v1/authorities/", 485, "Aberdeen", http.StatusOK},
		{"blank search returns full list", "/v1/authorities?search=", 485, "Aberdeen", http.StatusOK},
		{"whitespace search returns full list", "/v1/authorities?search=%20%20", 485, "Aberdeen", http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := do(t, srv, tc.path)
			if c.status != tc.wantStatus {
				t.Fatalf("status: got %d, want %d", c.status, tc.wantStatus)
			}
			if got, want := c.contentType, "application/json; charset=utf-8"; got != want {
				t.Errorf("content-type: got %q, want %q", got, want)
			}

			var list listResponse
			if err := json.Unmarshal(c.body, &list); err != nil {
				t.Fatalf("unmarshal %q: %v", c.body, err)
			}
			if list.Total != tc.wantTotal {
				t.Errorf("total: got %d, want %d", list.Total, tc.wantTotal)
			}
			if len(list.Authorities) != tc.wantTotal {
				t.Errorf("count: got %d, want %d", len(list.Authorities), tc.wantTotal)
			}
			if tc.wantFirst != "" && len(list.Authorities) > 0 && list.Authorities[0].Name != tc.wantFirst {
				t.Errorf("first: got %q, want %q", list.Authorities[0].Name, tc.wantFirst)
			}
		})
	}
}

func TestRoutes_AuthoritiesList_EmptySearchSerializesAsEmptyArray(t *testing.T) {
	t.Parallel()

	srv := newServer(t)
	c := do(t, srv, "/v1/authorities?search=ZZZNOMATCH")
	if c.status != http.StatusOK {
		t.Fatalf("status: got %d, want 200", c.status)
	}
	// The authorities array serialises as [] (not null) when no results match.
	if got, want := string(c.body), `{"authorities":[],"total":0}`; got != want {
		t.Errorf("body: got %s, want %s", got, want)
	}
}

func TestRoutes_AuthorityByID(t *testing.T) {
	t.Parallel()

	srv := newServer(t)

	t.Run("existing id returns full record with null urls", func(t *testing.T) {
		t.Parallel()
		c := do(t, srv, "/v1/authorities/384")
		if c.status != http.StatusOK {
			t.Fatalf("status: got %d, want 200", c.status)
		}
		if got, want := c.contentType, "application/json; charset=utf-8"; got != want {
			t.Errorf("content-type: got %q, want %q", got, want)
		}
		// councilUrl/planningUrl are always null (the embedded data never
		// populates them), in declaration order.
		if got, want := string(c.body), `{"id":384,"name":"Aberdeen","areaType":"Scottish Council","councilUrl":null,"planningUrl":null}`; got != want {
			t.Errorf("body: got %s, want %s", got, want)
		}
	})

	t.Run("valid int but missing id returns bodyless 404", func(t *testing.T) {
		t.Parallel()
		c := do(t, srv, "/v1/authorities/99999999")
		if c.status != http.StatusNotFound {
			t.Fatalf("status: got %d, want 404", c.status)
		}
		// Iteration 1: bodyless 404 (Results.NotFound). The PascalCase backfill
		// body is added by middleware in iteration 2.
		if len(c.body) != 0 {
			t.Errorf("body: got %q, want empty", c.body)
		}
	})
}

// TestRoutes_AuthorityNonIntID pins the contract for a non-integer id: Go's
// {id} wildcard matches any segment, so the handler self-denies non-ints with
// the same 401 challenge the auth middleware would emit (bodyless; the PascalCase
// envelope is backfilled by middleware.ErrorBody).
func TestRoutes_AuthorityNonIntID(t *testing.T) {
	t.Parallel()

	srv := newServer(t)

	for _, id := range []string{"abc", "1.5", "12x", "%20"} {
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			c := do(t, srv, "/v1/authorities/"+id)
			if c.status != http.StatusUnauthorized {
				t.Fatalf("status: got %d, want 401", c.status)
			}
			if len(c.body) != 0 {
				t.Errorf("body: got %q, want empty (backfilled downstream)", c.body)
			}
		})
	}
}
