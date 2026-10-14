// Package api serves the /api routes that sit outside the /v1 group. Today
// that is just GET /api/me, the authenticated identity probe the iOS app and
// web frontend call to resolve the current user's Auth0 subject.
package api

import (
	"log/slog"
	"net/http"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/httputil"
)

type userIDResponse struct {
	UserID string `json:"userId"`
}

// Routes registers the /api endpoints on mux. The route is authenticated: the
// auth middleware denies it without a valid token, so the handler can assume a
// subject is present in the request context.
func Routes(mux *http.ServeMux, logger *slog.Logger) {
	h := handler{logger: logger}
	mux.HandleFunc("GET /api/me", h.me)
}

type handler struct {
	logger *slog.Logger
}

func (h handler) me(w http.ResponseWriter, r *http.Request) {
	body, err := httputil.EncodeJSON(userIDResponse{UserID: auth.Subject(r.Context())})
	if err != nil {
		h.logger.ErrorContext(r.Context(), "encode me response", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if _, err := w.Write(body); err != nil {
		h.logger.ErrorContext(r.Context(), "write me response", "error", err)
	}
}
