package watchzones

import (
	"context"
	"log/slog"
	"net/http"
	"slices"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/authorities"
	"github.com/AmyDe/town-crier/api-go/internal/httputil"
)

// zoneAuthorityLister yields the user's watch zones; the application-authorities
// endpoint derives its authority set from their distinct authority ids.
type zoneAuthorityLister interface {
	GetByUserID(ctx context.Context, userID string) ([]WatchZone, error)
}

// authorityLookup resolves an authority id to its display metadata.
type authorityLookup interface {
	ByID(id int) (authorities.Authority, bool)
}

// authoritiesHandler serves GET /v1/me/application-authorities.
type authoritiesHandler struct {
	zones  zoneAuthorityLister
	lookup authorityLookup
	logger *slog.Logger
}

// AuthoritiesRoutes registers GET /v1/me/application-authorities, the distinct
// set of authorities across the user's watch zones, resolved to display
// metadata and sorted by name. It lives in the watchzones package because the
// result is derived purely from the caller's watch zones — it never reads a
// planning application.
func AuthoritiesRoutes(mux *http.ServeMux, zones zoneAuthorityLister, lookup authorityLookup, logger *slog.Logger) {
	h := &authoritiesHandler{zones: zones, lookup: lookup, logger: logger}
	mux.HandleFunc("GET /v1/me/application-authorities", h.list)
}

// authorityItem is the wire shape for one authority: { id, name, areaType }.
type authorityItem struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	AreaType string `json:"areaType"`
}

// applicationAuthoritiesResult is the wire response: { authorities: [...], count }.
type applicationAuthoritiesResult struct {
	Authorities []authorityItem `json:"authorities"`
	Count       int             `json:"count"`
}

// list resolves the distinct authorities across the user's watch zones to
// display metadata, sorts them by name (ordinal, case-insensitive) and returns
// the list with its count. Authorities not present in the static data are
// silently skipped.
func (h *authoritiesHandler) list(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())

	zones, err := h.zones.GetByUserID(r.Context(), userID)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "watch-zone request failed", "op", "list watch zones", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	seen := make(map[int]struct{}, len(zones))
	items := []authorityItem{}
	for _, z := range zones {
		if _, dup := seen[z.AuthorityID]; dup {
			continue
		}
		seen[z.AuthorityID] = struct{}{}
		if a, ok := h.lookup.ByID(z.AuthorityID); ok {
			items = append(items, authorityItem{ID: a.ID, Name: a.Name, AreaType: a.AreaType})
		}
	}

	slices.SortStableFunc(items, func(a, b authorityItem) int {
		return authorities.CompareOrdinalIgnoreCase(a.Name, b.Name)
	})

	body, err := httputil.EncodeJSON(applicationAuthoritiesResult{Authorities: items, Count: len(items)})
	if err != nil {
		h.logger.ErrorContext(r.Context(), "watch-zone request failed", "op", "encode authorities", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(body); err != nil {
		h.logger.ErrorContext(r.Context(), "write response", "error", err)
	}
}
