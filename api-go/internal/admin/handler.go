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
	"github.com/AmyDe/town-crier/api-go/internal/subscriptionevents"
)

const maxBodyBytes = 1 << 20

// profileAdminStore is the cross-partition profile store the admin handlers use.
type profileAdminStore interface {
	GetByEmail(ctx context.Context, email string) (*profiles.UserProfile, error)
	Save(ctx context.Context, p *profiles.UserProfile) error
	List(ctx context.Context, emailSearch string, pageSize int, continuationToken string) (profiles.Page, error)
	PaidCandidates(ctx context.Context) ([]*profiles.UserProfile, error)
	UserStats(ctx context.Context, now time.Time) (profiles.UserStats, error)
}

// tierSync pushes the tier into Auth0's app_metadata.
type tierSync interface {
	UpdateSubscriptionTier(ctx context.Context, userID, tier string) error
}

type notificationCounts interface {
	CountsByUsers(ctx context.Context, userIDs []string) (map[string]notifications.NotificationCounts, error)
	Totals(ctx context.Context) (notifications.NotificationTotals, error)
}

type savedCountReader interface {
	CountsByUsers(ctx context.Context, userIDs []string) (map[string]int, error)
	Count(ctx context.Context) (int, error)
}

type deviceCountReader interface {
	CountsByUsers(ctx context.Context, userIDs []string) (map[string]int, error)
	Count(ctx context.Context) (int, error)
}

type offerRedemptionReader interface {
	RedeemedByUsers(ctx context.Context, userIDs []string) (map[string][]offercodes.RedeemedOfferCode, error)
}

type offerCodeStore interface {
	Save(ctx context.Context, c offercodes.OfferCode) error
	List(ctx context.Context, labelFilter *string, limit int) ([]offercodes.ListedOfferCode, error)
}

type codeGenerator interface {
	Generate() (string, error)
}

type subscriptionNotifier interface {
	Changed(ctx context.Context, before, after subscriptionevents.Snapshot, source subscriptionevents.Source)
}

type handler struct {
	profiles     profileAdminStore
	notifCounts  notificationCounts
	savedCounts  savedCountReader
	deviceCounts deviceCountReader
	redemptions  offerRedemptionReader
	auth0        tierSync
	notifier     subscriptionNotifier
	codes        offerCodeStore
	generator    codeGenerator
	now          func() time.Time
	logger       *slog.Logger
}

// Routes registers the admin endpoints on mux. The routes are anonymous to Auth0,
// so the admin key gate is their only authentication. The notifCounts,
// savedCounts, deviceCounts and redemptions readers may be nil; a nil reader
// means "metric absent".
func Routes(mux *http.ServeMux, adminKey string, profileStore profileAdminStore, notifCounts notificationCounts, savedCounts savedCountReader, deviceCounts deviceCountReader, redemptions offerRedemptionReader, auth0 tierSync, notifier subscriptionNotifier, codes offerCodeStore, generator codeGenerator, now func() time.Time, logger *slog.Logger) {
	h := &handler{
		profiles:     profileStore,
		notifCounts:  notifCounts,
		savedCounts:  savedCounts,
		deviceCounts: deviceCounts,
		redemptions:  redemptions,
		auth0:        auth0,
		notifier:     notifier,
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
