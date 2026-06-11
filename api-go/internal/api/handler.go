// Package api serves the /api routes that sit outside the /v1 group. Today
// that is just GET /api/me, the authenticated identity probe the iOS app and
// web frontend call to resolve the current user's Auth0 subject.
package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
)

// userIDResponse mirrors the .NET UserIdResponse record. The JSON key is
// userId — explicit camelCase via [JsonPropertyName] — not the PascalCase the
// field name would otherwise produce.
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
	// The subject was injected by the auth middleware after validating the
	// bearer token; .NET reads the same value via ClaimsPrincipal sub claim.
	resp := userIDResponse{UserID: auth.Subject(r.Context())}

	// Encode through a buffer with HTML escaping off (matching .NET) and trim
	// the trailing newline so the wire bytes are compact and identical.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(resp); err != nil {
		h.logger.ErrorContext(r.Context(), "encode me response", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// charset=utf-8 matches ASP.NET Core's Results.Ok JSON byte-for-byte.
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if _, err := w.Write(bytes.TrimRight(buf.Bytes(), "\n")); err != nil {
		h.logger.ErrorContext(r.Context(), "write me response", "error", err)
	}
}
