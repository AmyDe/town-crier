package admin

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/httputil"
	"github.com/AmyDe/town-crier/api-go/internal/offercodes"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

const maxBodyBytes = 1 << 20

// profileAdminStore is the cross-partition profile store the admin handlers use:
// find-by-email and save (grant) plus the paged list. profiles.AdminStore
// satisfies it.
type profileAdminStore interface {
	GetByEmail(ctx context.Context, email string) (*profiles.UserProfile, error)
	Save(ctx context.Context, p *profiles.UserProfile) error
	List(ctx context.Context, emailSearch string, pageSize int, continuationToken string) (profiles.Page, error)
}

// tierSync is the Auth0 management interface the admin handlers use: push the
// tier into Auth0's app_metadata. profiles.Auth0Manager satisfies it.
type tierSync interface {
	UpdateSubscriptionTier(ctx context.Context, userID, tier string) error
}

// offerCodeStore is the offer-code writer the generate endpoint uses.
// offercodes.CosmosStore satisfies it.
type offerCodeStore interface {
	Save(ctx context.Context, c offercodes.OfferCode) error
}

// codeGenerator mints fresh canonical codes. offercodes.RandomGenerator
// satisfies it.
type codeGenerator interface {
	Generate() (string, error)
}

type handler struct {
	profiles  profileAdminStore
	auth0     tierSync
	codes     offerCodeStore
	generator codeGenerator
	now       func() time.Time
	logger    *slog.Logger
}

// Routes registers the admin endpoints on mux, each gated by the shared admin
// key. The routes are anonymous to Auth0 (the caller carries no bearer token),
// so the key gate is their only authentication.
func Routes(mux *http.ServeMux, adminKey string, profileStore profileAdminStore, auth0 tierSync, codes offerCodeStore, generator codeGenerator, now func() time.Time, logger *slog.Logger) {
	h := &handler{profiles: profileStore, auth0: auth0, codes: codes, generator: generator, now: now, logger: logger}
	mux.HandleFunc("PUT /v1/admin/subscriptions", requireAdminKey(adminKey, h.grantSubscription))
	mux.HandleFunc("GET /v1/admin/users", requireAdminKey(adminKey, h.listUsers))
	mux.HandleFunc("POST /v1/admin/offer-codes", requireAdminKey(adminKey, h.generateOfferCodes))
}

func (h *handler) writeJSON(r *http.Request, w http.ResponseWriter, v any) {
	body, err := httputil.EncodeJSON(v)
	if err != nil {
		h.serverError(w, r, "encode response", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(body); err != nil {
		h.logger.ErrorContext(r.Context(), "write admin response", "error", err)
	}
}

func (h *handler) serverError(w http.ResponseWriter, r *http.Request, op string, err error) {
	h.logger.ErrorContext(r.Context(), "admin request failed", "op", op, "error", err)
	w.WriteHeader(http.StatusInternalServerError)
}
