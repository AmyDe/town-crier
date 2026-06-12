package devicetokens

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
)

// registrationStore is the consumer-side slice of the store the handlers use.
// *CosmosStore satisfies it; tests substitute a hand-written fake.
type registrationStore interface {
	GetByToken(ctx context.Context, userID, token string) (*DeviceRegistration, error)
	Save(ctx context.Context, reg DeviceRegistration) error
	Delete(ctx context.Context, userID, token string) error
}

// registerRequest mirrors .NET RegisterDeviceTokenRequest. Platform arrives as
// the string-enum form ("Ios"/"Android", bound case-insensitively).
type registerRequest struct {
	Token    string `json:"token"`
	Platform string `json:"platform"`
}

// Routes registers the device-token endpoints on mux. Both are authenticated:
// the auth middleware guarantees a subject in context before these run.
func Routes(mux *http.ServeMux, store registrationStore, now func() time.Time, logger *slog.Logger) {
	h := handler{store: store, now: now, logger: logger}
	mux.HandleFunc("PUT /v1/me/device-token", h.register)
	mux.HandleFunc("DELETE /v1/me/device-token/{token}", h.remove)
}

type handler struct {
	store  registrationStore
	now    func() time.Time
	logger *slog.Logger
}

// register PUTs a device token: refresh the registration instant when the
// (user, token) pair already exists, create it otherwise. 204 on success,
// mirroring .NET RegisterDeviceToken -> Results.NoContent(). A malformed body
// or unknown platform fails JSON binding in .NET and yields a bodyless 400
// (backfilled by the error middleware) — reproduced here.
func (h handler) register(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	platform, err := ParsePlatform(req.Platform)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	reg, err := NewRegistration(userID, req.Token, platform, h.now())
	if err != nil {
		// .NET reaches the domain guard only after binding succeeds, so a blank
		// token surfaces as an unhandled ArgumentException -> 500. Mirror the
		// status; the Detail string is .NET-internal and not contract-pinned.
		h.logger.ErrorContext(r.Context(), "register device token", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	existing, err := h.store.GetByToken(r.Context(), userID, req.Token)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "read device token", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if existing != nil {
		existing.Refresh(h.now())
		reg = *existing
	}
	if err := h.store.Save(r.Context(), reg); err != nil {
		h.logger.ErrorContext(r.Context(), "save device token", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// remove DELETEs a (user, token) registration. Idempotent: removing an absent
// token still returns 204, mirroring .NET RemoveInvalidDeviceToken.
func (h handler) remove(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())
	token := r.PathValue("token")

	if err := h.store.Delete(r.Context(), userID, token); err != nil {
		h.logger.ErrorContext(r.Context(), "delete device token", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
