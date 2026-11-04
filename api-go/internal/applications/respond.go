package applications

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/AmyDe/town-crier/api-go/internal/authorities"
	"github.com/AmyDe/town-crier/api-go/internal/httputil"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
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
	if _, err := w.Write(body); err != nil { //nolint:gosec // JSON body, application/json content type
		logger.ErrorContext(r.Context(), "write response", "error", err)
	}
}

// serverError answers a bodyless error status chosen by platform.ClassifyRequestError:
// 499 (not logged) for a cancelled request, 503 for a timeout, otherwise 500.
// middleware.ErrorBody backfills the envelope.
func serverError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, op string, err error) {
	platform.RespondRequestError(w, r, logger, "application request failed", err, "op", op)
}

// resolveAuthoritySlugFor prefers resolver.SlugForAreaID so the slug
// round-trips through SlugToAreaID. For an area id missing from the static
// authorities data it falls back to slugifying the area name and warn-logs,
// since a link built from that slug may 404. op names the caller in the log.
func resolveAuthoritySlugFor(ctx context.Context, resolver authoritySlugResolver, logger *slog.Logger, op string, app PlanningApplication) string {
	if slug, ok := resolver.SlugForAreaID(app.AreaID); ok {
		return slug
	}
	logger.WarnContext(ctx, "authority slug fallback: area id not in static authorities", "op", op, "areaId", app.AreaID, "areaName", app.AreaName, "uid", app.UID)
	return authorities.Slugify(app.AreaName)
}
