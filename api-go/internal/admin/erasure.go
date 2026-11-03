package admin

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/AmyDe/town-crier/api-go/internal/erasure"
	"github.com/AmyDe/town-crier/api-go/internal/httputil"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// maxErasuresPerEmail bounds the email loop so a profile delete that silently
// leaves its row behind cannot spin the handler forever.
const maxErasuresPerEmail = 20

type erasureProfileFinder interface {
	GetByEmail(ctx context.Context, email string) (*profiles.UserProfile, error)
}

type erasureHandler struct {
	finder   erasureProfileFinder
	deleters erasure.Deleters
	logger   *slog.Logger
}

type eraseBody struct {
	Email  string `json:"email"`
	UserID string `json:"userId"`
}

type eraseResult struct {
	UserIDs []string `json:"userIds"`
}

type eraseFailure struct {
	ErasedUserIDs []string `json:"erasedUserIds"`
	FailedUserID  string   `json:"failedUserId,omitempty"`
}

// ErasureRoutes registers POST /v1/admin/users/erase behind the shared admin
// key. It runs the full UK GDPR Art. 17 erasure (erasure.Cascade, the same
// cascade as DELETE /v1/me). The body names exactly one target:
//
//   - {"email": "..."} erases every profile whose email matches exactly after
//     trimming whitespace (email is not unique in users), newest first, until
//     none is left. The email travels in the body so it stays out of request
//     URLs and telemetry.
//   - {"userId": "..."} runs the cascade for that user id whether or not a
//     profile exists, so it finishes an erasure that failed after the profile
//     step (only the Auth0 user left). Every step tolerates absent data, so a
//     repeat is safe.
//
// Responses: 200 {"userIds": [...]} after every erasure completes; 400 unless
// exactly one of email and userId is non-blank; 404 when no profile has the
// email; 500 {"erasedUserIds": [...], "failedUserId": "..."} when a step
// fails, where failedUserId is absent if the email lookup itself failed.
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
	email, userID := strings.TrimSpace(req.Email), strings.TrimSpace(req.UserID)
	if (email == "") == (userID == "") {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if userID != "" {
		if err := h.cascade(r.Context(), userID); err != nil {
			h.writeJSON(w, r, http.StatusInternalServerError, eraseFailure{ErasedUserIDs: []string{}, FailedUserID: userID})
			return
		}
		h.writeJSON(w, r, http.StatusOK, eraseResult{UserIDs: []string{userID}})
		return
	}

	erased := []string{}
	for range maxErasuresPerEmail {
		profile, err := h.finder.GetByEmail(r.Context(), email)
		if errors.Is(err, profiles.ErrNotFound) {
			break
		}
		if err != nil {
			h.logger.ErrorContext(r.Context(), "admin erasure failed", "op", "find profile by email", "erasedUserIds", erased, "error", err)
			h.writeJSON(w, r, http.StatusInternalServerError, eraseFailure{ErasedUserIDs: erased})
			return
		}
		if slices.Contains(erased, profile.UserID) {
			h.logger.ErrorContext(r.Context(), "admin erasure failed", "op", "profile survived erasure", "userId", profile.UserID)
			h.writeJSON(w, r, http.StatusInternalServerError, eraseFailure{ErasedUserIDs: erased, FailedUserID: profile.UserID})
			return
		}
		if err := h.cascade(r.Context(), profile.UserID); err != nil {
			h.writeJSON(w, r, http.StatusInternalServerError, eraseFailure{ErasedUserIDs: erased, FailedUserID: profile.UserID})
			return
		}
		erased = append(erased, profile.UserID)
	}
	if len(erased) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	h.writeJSON(w, r, http.StatusOK, eraseResult{UserIDs: erased})
}

func (h *erasureHandler) cascade(ctx context.Context, userID string) error {
	if err := erasure.Cascade(ctx, userID, h.deleters); err != nil {
		h.logger.ErrorContext(ctx, "admin erasure failed", "op", "erase account", "userId", userID, "error", err)
		return err
	}
	h.logger.InfoContext(ctx, "admin.user_erased", "userId", userID)
	return nil
}

func (h *erasureHandler) writeJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	body, err := httputil.EncodeJSON(v)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "admin erasure failed", "op", "encode response", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		h.logger.ErrorContext(r.Context(), "write admin response", "error", err)
	}
}
