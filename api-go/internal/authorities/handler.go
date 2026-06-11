package authorities

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

// authorityStore is the consumer-side view the handler needs. The concrete
// *staticStore satisfies it structurally; tests can substitute a fake.
type authorityStore interface {
	all() []Authority
	byID(id int) (Authority, bool)
}

// listItem is one entry in the GET /v1/authorities response. Field order
// matches the .NET AuthorityListItem record so the wire bytes are identical.
type listItem struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	AreaType string `json:"areaType"`
}

// listResult mirrors the .NET GetAuthoritiesResult record: a non-null array
// plus a total count.
type listResult struct {
	Authorities []listItem `json:"authorities"`
	Total       int        `json:"total"`
}

// detailResult mirrors the .NET GetAuthorityByIdResult record. councilUrl and
// planningUrl are always null because the embedded data never populates them,
// but the fields are emitted explicitly for parity. Pointers serialize a nil
// as JSON null.
type detailResult struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	AreaType    string  `json:"areaType"`
	CouncilURL  *string `json:"councilUrl"`
	PlanningURL *string `json:"planningUrl"`
}

// Routes registers the authorities endpoints on mux.
func Routes(mux *http.ServeMux, logger *slog.Logger) {
	h := handler{store: newStaticStore(), logger: logger}
	mux.HandleFunc("GET /v1/authorities", h.list)
	// Trailing slash: ASP.NET Core routing is trailing-slash-insensitive and
	// serves the list for GET /v1/authorities/ . Go's mux treats it as a
	// distinct path that {id} will not match, so register it explicitly.
	mux.HandleFunc("GET /v1/authorities/{$}", h.list)
	mux.HandleFunc("GET /v1/authorities/{id}", h.byID)
}

type handler struct {
	store  authorityStore
	logger *slog.Logger
}

func (h handler) list(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("search")

	all := h.store.all()
	items := make([]listItem, 0, len(all))
	// .NET filters with !IsNullOrWhiteSpace(search): a blank or whitespace-only
	// search applies no filter and returns the full list.
	filter := strings.TrimSpace(search) != ""
	for _, a := range all {
		if filter && !containsOrdinalIgnoreCase(a.Name, search) {
			continue
		}
		items = append(items, listItem(a))
	}

	h.writeJSON(r, w, listResult{Authorities: items, Total: len(items)})
}

func (h handler) byID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		// Non-integer id: the .NET {id:int} route constraint does not match, so
		// the request falls through to the auth fallback (a 401 added by auth
		// middleware in iteration 2). Until that lands, the closest honest
		// behaviour is a 404 (backfilled by middleware.ErrorBody); the e2e
		// harness defers the 401 scenario to iteration 2.
		w.WriteHeader(http.StatusNotFound)
		return
	}

	a, ok := h.store.byID(id)
	if !ok {
		// Parity: .NET returns Results.NotFound() (bodyless); the PascalCase
		// envelope is backfilled by middleware.ErrorBody, as in .NET.
		w.WriteHeader(http.StatusNotFound)
		return
	}

	h.writeJSON(r, w, detailResult{ID: a.ID, Name: a.Name, AreaType: a.AreaType})
}

func (h handler) writeJSON(r *http.Request, w http.ResponseWriter, v any) {
	// Encode through a buffer with HTML escaping off (matching .NET) and trim
	// the trailing newline json.Encoder appends, so the wire bytes are compact
	// and identical to the .NET response.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		h.logger.ErrorContext(r.Context(), "encode authorities response", "path", r.URL.Path, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if _, err := w.Write(bytes.TrimRight(buf.Bytes(), "\n")); err != nil {
		h.logger.ErrorContext(r.Context(), "write authorities response", "path", r.URL.Path, "error", err)
	}
}

// containsOrdinalIgnoreCase mirrors string.Contains(value, search,
// StringComparison.OrdinalIgnoreCase) for ASCII data: a case-insensitive
// substring test.
func containsOrdinalIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToUpper(s), strings.ToUpper(substr))
}
