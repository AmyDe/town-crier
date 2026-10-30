package devicetokens

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// registrationStore is the consumer-side slice of the store the handlers use.
type registrationStore interface {
	GetByToken(ctx context.Context, userID, token string) (*DeviceRegistration, error)
	Save(ctx context.Context, reg DeviceRegistration) error
	Delete(ctx context.Context, userID, token string) error
}

// registerRequest is the JSON body for PUT /v1/me/device-token. Platform
// arrives as the string-enum form ("Ios"/"Android", matched case-insensitively).
type registerRequest struct {
	Token    string `json:"token"`
	Platform string `json:"platform"`
}

// removeRequest is the JSON body for DELETE /v1/me/device-token.
type removeRequest struct {
	Token string `json:"token"`
}

// Routes registers the device-token endpoints on mux. All are authenticated:
// the auth middleware guarantees a subject in context before these run.
func Routes(mux *http.ServeMux, store registrationStore, now func() time.Time, logger *slog.Logger) {
	h := handler{store: store, now: now, logger: logger}
	mux.HandleFunc("PUT /v1/me/device-token", h.register)
	mux.HandleFunc("DELETE /v1/me/device-token", h.removeByBody)
	mux.HandleFunc("DELETE /v1/me/device-token/{token}", h.remove)
}

type handler struct {
	store  registrationStore
	now    func() time.Time
	logger *slog.Logger
}

// register PUTs a device token: refresh the registration instant when the
// (user, token) pair already exists, create it otherwise. 204 on success.
// A malformed body or unknown platform yields a bodyless 400 (backfilled by
// the error middleware).
func (h handler) register(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	devicePlatform, err := ParsePlatform(req.Platform)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	reg, err := NewRegistration(userID, req.Token, devicePlatform, h.now())
	if err != nil {
		// A blank token only reaches this guard after binding succeeds (a blank is
		// technically valid JSON), so it surfaces as a 500.
		h.logger.ErrorContext(r.Context(), "register device token", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	existing, err := h.store.GetByToken(r.Context(), userID, req.Token)
	if err != nil {
		platform.RespondRequestError(w, r, h.logger, "read device token", err)
		return
	}
	if existing != nil {
		existing.Refresh(h.now())
		reg = *existing
	}
	if err := h.store.Save(r.Context(), reg); err != nil {
		platform.RespondRequestError(w, r, h.logger, "save device token", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// removeByBody DELETEs a (user, token) registration named in the JSON body.
// Idempotent like remove. A malformed body or blank token yields a bodyless 400.
func (h handler) removeByBody(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req removeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	h.delete(w, r, req.Token)
}

// remove DELETEs a (user, token) registration named in the path. Idempotent:
// removing an absent token still returns 204.
func (h handler) remove(w http.ResponseWriter, r *http.Request) {
	h.delete(w, r, r.PathValue("token"))
}

func (h handler) delete(w http.ResponseWriter, r *http.Request, token string) {
	userID := auth.Subject(r.Context())
	if err := h.store.Delete(r.Context(), userID, token); err != nil {
		platform.RespondRequestError(w, r, h.logger, "delete device token", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
