package watchzones

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// filterCatalogEntry is the wire shape for one catalog filter. It
// deliberately carries no Expression (or any other matching-internals)
// field: the to_tsquery expression is server-only implementation detail,
// never served to a client.
type filterCatalogEntry struct {
	Key         string `json:"key"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
}

// filterCatalogResult is the response body for GET
// /v1/watch-zones/filter-catalog.
type filterCatalogResult struct {
	Filters []filterCatalogEntry `json:"filters"`
}

// CatalogRoutes registers the read-only, anonymous watch-zone filter catalog
// endpoint on mux. It needs no store: the catalog is Go-literal static
// content (see filter.go), so this is wired unconditionally regardless of
// whether a watch-zone store is configured (GH#1104, tc-m8j90.1).
func CatalogRoutes(mux *http.ServeMux, logger *slog.Logger) {
	h := catalogHandler{logger: logger}
	mux.HandleFunc("GET /v1/watch-zones/filter-catalog", h.catalog)
}

type catalogHandler struct {
	logger *slog.Logger
}

// catalog always returns 200: the catalog is static content baked into the
// binary, so there is no not-found or error branch to model.
func (h catalogHandler) catalog(w http.ResponseWriter, r *http.Request) {
	result := filterCatalogResult{Filters: make([]filterCatalogEntry, 0, len(filterCatalogOrder))}
	for _, key := range filterCatalogOrder {
		def, ok := FilterDefinitionFor(key)
		if !ok {
			// Unreachable unless filterCatalogOrder and filterCatalog drift out of
			// sync; skip rather than fail the whole response for the other entries.
			continue
		}
		result.Filters = append(result.Filters, filterCatalogEntry{
			Key:         string(key),
			DisplayName: def.DisplayName,
			Description: def.Description,
		})
	}

	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		h.logger.ErrorContext(r.Context(), "write filter catalog response", "error", err)
	}
}
