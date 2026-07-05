package applications

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/AmyDe/town-crier/api-go/internal/authorities"
	"github.com/AmyDe/town-crier/api-go/internal/httputil"
)

// writeJSON encodes v as a 200 application/json; charset=utf-8 response with HTML
// escaping off and no trailing newline.
func writeJSON(w http.ResponseWriter, r *http.Request, logger *slog.Logger, v any) {
	body, err := httputil.EncodeJSON(v)
	if err != nil {
		serverError(w, r, logger, "encode response", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	// gosec G705: body is JSON from httputil.EncodeJSON served as
	// application/json, so it is never interpreted as HTML — no XSS surface.
	if _, err := w.Write(body); err != nil { //nolint:gosec // JSON body, application/json content type
		logger.ErrorContext(r.Context(), "write response", "error", err)
	}
}

// serverError logs and emits a bodyless 500; the error envelope is backfilled by
// middleware.ErrorBody.
func serverError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, op string, err error) {
	logger.ErrorContext(r.Context(), "application request failed", "op", op, "error", err)
	w.WriteHeader(http.StatusInternalServerError)
}

// resolveAuthoritySlugFor resolves the URL slug for an application's authority,
// round-trip-safe against SlugToAreaID: it prefers resolver.SlugForAreaID (so
// the emitted slug is exactly what SlugToAreaID resolves back), and only when
// the id is unknown falls back to slugifying the PlanIt area name. The
// fallback is warn-logged because it means PlanIt returned an area id absent
// from the static authorities data — the emitted slug then may not round-trip
// back through SlugToAreaID, so a share/by-slug link built from it could 404.
// op names the calling handler in the log line (each handler passes its own,
// distinct string) so a fallback can be traced back to its endpoint. Shared by
// (h *handler).authoritySlug and (h *searchHandler).authoritySlug.
func resolveAuthoritySlugFor(ctx context.Context, resolver authoritySlugResolver, logger *slog.Logger, op string, app PlanningApplication) string {
	if slug, ok := resolver.SlugForAreaID(app.AreaID); ok {
		return slug
	}
	logger.WarnContext(ctx, "authority slug fallback: area id not in static authorities", "op", op, "areaId", app.AreaID, "areaName", app.AreaName, "uid", app.UID)
	return authorities.Slugify(app.AreaName)
}
