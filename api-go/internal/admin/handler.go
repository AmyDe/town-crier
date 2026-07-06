package admin

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/httputil"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/offercodes"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

const maxBodyBytes = 1 << 20

// profileAdminStore is the cross-partition profile store the admin handlers use:
// find-by-email and save (grant), the paged list, and the stats aggregates
// (paid-tier candidates + the whole-base UserStats). profiles.PostgresAdminStore
// satisfies it.
type profileAdminStore interface {
	GetByEmail(ctx context.Context, email string) (*profiles.UserProfile, error)
	Save(ctx context.Context, p *profiles.UserProfile) error
	List(ctx context.Context, emailSearch string, pageSize int, continuationToken string) (profiles.Page, error)
	PaidCandidates(ctx context.Context) ([]*profiles.UserProfile, error)
	UserStats(ctx context.Context, now time.Time) (profiles.UserStats, error)
}

// tierSync is the Auth0 management interface the admin handlers use: push the
// tier into Auth0's app_metadata. profiles.Auth0Manager satisfies it.
type tierSync interface {
	UpdateSubscriptionTier(ctx context.Context, userID, tier string) error
}

// notificationCounts is the notification store the admin surface reads: the
// batched per-user tally for the user list and the whole-table totals for the
// stats reach block. *notifications.PostgresStore satisfies it.
type notificationCounts interface {
	CountsByUsers(ctx context.Context, userIDs []string) (map[string]notifications.NotificationCounts, error)
	Totals(ctx context.Context) (notifications.NotificationTotals, error)
}

// savedCountReader is the saved-application store the admin surface reads: the
// batched per-user count for the user list and the global total for the stats
// reach block. *savedapplications.PostgresStore satisfies it.
type savedCountReader interface {
	CountsByUsers(ctx context.Context, userIDs []string) (map[string]int, error)
	Count(ctx context.Context) (int, error)
}

// deviceCountReader is the device-registration store the admin surface reads:
// the batched per-user count for the user list and the global total for the
// stats reach block. *devicetokens.PostgresStore satisfies it.
type deviceCountReader interface {
	CountsByUsers(ctx context.Context, userIDs []string) (map[string]int, error)
	Count(ctx context.Context) (int, error)
}

// offerRedemptionReader is the batched offer-code redemption reader the user
// list uses to surface each user's active offer code.
// *offercodes.PostgresStore satisfies it.
type offerRedemptionReader interface {
	RedeemedByUsers(ctx context.Context, userIDs []string) (map[string][]offercodes.RedeemedOfferCode, error)
}

// offerCodeStore is the offer-code writer/reader the generate and list
// endpoints use. offercodes.PostgresStore satisfies it.
type offerCodeStore interface {
	Save(ctx context.Context, c offercodes.OfferCode) error
	List(ctx context.Context, labelFilter *string, limit int) ([]offercodes.ListedOfferCode, error)
}

// codeGenerator mints fresh canonical codes. offercodes.RandomGenerator
// satisfies it.
type codeGenerator interface {
	Generate() (string, error)
}

type handler struct {
	profiles     profileAdminStore
	notifCounts  notificationCounts
	savedCounts  savedCountReader
	deviceCounts deviceCountReader
	redemptions  offerRedemptionReader
	auth0        tierSync
	codes        offerCodeStore
	generator    codeGenerator
	now          func() time.Time
	logger       *slog.Logger
}

// Routes registers the admin endpoints on mux, each gated by the shared admin
// key. The routes are anonymous to Auth0 (the caller carries no bearer token),
// so the key gate is their only authentication. The enrichment/reach readers
// (notifCounts, savedCounts, deviceCounts, redemptions) may be nil on a
// store-less local boot; the handlers treat a nil reader as "metric absent".
func Routes(mux *http.ServeMux, adminKey string, profileStore profileAdminStore, notifCounts notificationCounts, savedCounts savedCountReader, deviceCounts deviceCountReader, redemptions offerRedemptionReader, auth0 tierSync, codes offerCodeStore, generator codeGenerator, now func() time.Time, logger *slog.Logger) {
	h := &handler{
		profiles:     profileStore,
		notifCounts:  notifCounts,
		savedCounts:  savedCounts,
		deviceCounts: deviceCounts,
		redemptions:  redemptions,
		auth0:        auth0,
		codes:        codes,
		generator:    generator,
		now:          now,
		logger:       logger,
	}
	mux.HandleFunc("PUT /v1/admin/subscriptions", requireAdminKey(adminKey, h.grantSubscription))
	mux.HandleFunc("GET /v1/admin/users", requireAdminKey(adminKey, h.listUsers))
	mux.HandleFunc("GET /v1/admin/stats", requireAdminKey(adminKey, h.stats))
	mux.HandleFunc("POST /v1/admin/offer-codes", requireAdminKey(adminKey, h.generateOfferCodes))
	mux.HandleFunc("GET /v1/admin/offer-codes", requireAdminKey(adminKey, h.listOfferCodes))
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
