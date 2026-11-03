package admin

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/AmyDe/town-crier/api-go/internal/erasure"
	"github.com/AmyDe/town-crier/api-go/internal/httputil"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

type erasureProfileFinder interface {
	GetByEmail(ctx context.Context, email string) (*profiles.UserProfile, error)
}

type erasureHandler struct {
	finder   erasureProfileFinder
	deleters erasure.Deleters
	logger   *slog.Logger
}

type eraseBody struct {
	Email string `json:"email"`
}

type eraseResult struct {
	UserID string `json:"userId"`
}

// ErasureRoutes registers POST /v1/admin/users/erase behind the shared admin
// key. It runs the full UK GDPR Art. 17 erasure (erasure.Cascade, the same
// cascade as DELETE /v1/me) for the profile whose email matches the body
// {"email": "..."} exactly, after trimming whitespace. The email travels in the
// body so it stays out of request URLs and telemetry.
//
// Responses: 200 {"userId": "..."} after a complete erasure; 400 for a missing
// or blank email; 404 when no profile has that email; 500 when a step fails.
// A 500 after the profile step leaves only the Auth0 user, and a retry by email
// then returns 404, so the handler logs the user id of every failure.
func ErasureRoutes(mux *http.ServeMux, adminKey string, finder erasureProfileFinder, deleters erasure.Deleters, logger *slog.Logger) {
	h := &erasureHandler{finder: finder, deleters: deleters, logger: logger}
	mux.HandleFunc("POST /v1/admin/users/erase", requireAdminKey(adminKey, h.erase))
}

func (h *erasureHandler) erase(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req eraseBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	email := strings.TrimSpace(req.Email)
	if email == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	profile, err := h.finder.GetByEmail(r.Context(), email)
	if err != nil {
		if errors.Is(err, profiles.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		h.logger.ErrorContext(r.Context(), "admin erasure failed", "op", "find profile by email", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := erasure.Cascade(r.Context(), profile.UserID, h.deleters); err != nil {
		h.logger.ErrorContext(r.Context(), "admin erasure failed", "op", "erase account", "userId", profile.UserID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	h.logger.InfoContext(r.Context(), "admin.user_erased", "userId", profile.UserID)

	body, err := httputil.EncodeJSON(eraseResult{UserID: profile.UserID})
	if err != nil {
		h.logger.ErrorContext(r.Context(), "admin erasure failed", "op", "encode response", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(body); err != nil {
		h.logger.ErrorContext(r.Context(), "write admin response", "error", err)
	}
}
