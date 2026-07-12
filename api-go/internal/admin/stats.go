package admin

import (
	"context"
	"net/http"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// statsResponse is the pinned JSON contract for GET /v1/admin/stats. The field
// order and names are load-bearing: the tc CLI mirrors this shape byte-for-field.
type statsResponse struct {
	Users    statsUsers    `json:"users"`
	Paying   statsPaying   `json:"paying"`
	Signups  statsSignups  `json:"signups"`
	Activity statsActivity `json:"activity"`
	Reach    statsReach    `json:"reach"`
}

type statsUsers struct {
	Total  int         `json:"total"`
	ByTier statsByTier `json:"byTier"`
}

// statsByTier is an explicit struct (not a map) so the three tier keys always
// render, in a fixed order, even when a tier has zero users.
type statsByTier struct {
	Free     int `json:"Free"`
	Personal int `json:"Personal"`
	Pro      int `json:"Pro"`
}

type statsPaying struct {
	EffectivePaid  int                 `json:"effectivePaid"`
	AppStore       int                 `json:"appStore"`
	Comped         int                 `json:"comped"`
	Lapsed         int                 `json:"lapsed"`
	InGrace        int                 `json:"inGrace"`
	AppStoreByTier statsAppStoreByTier `json:"appStoreByTier"`
}

// statsAppStoreByTier is an explicit struct (not a map) so the two paid tier
// keys always render, in a fixed order, even when a tier has zero App
// Store-backed payers. It is a subset of statsByTier: Free is never a paid
// tier, so it has no place here.
type statsAppStoreByTier struct {
	Personal int `json:"Personal"`
	Pro      int `json:"Pro"`
}

type statsSignups struct {
	Last24h    int              `json:"last24h"`
	Last7d     int              `json:"last7d"`
	Last30d    int              `json:"last30d"`
	MostRecent *statsMostRecent `json:"mostRecent"`
}

// statsMostRecent is null when the user base is empty; Email is null when the
// most-recent account has none (e.g. a Sign-in-with-Apple withheld email).
type statsMostRecent struct {
	UserID    string  `json:"userId"`
	Email     *string `json:"email"`
	CreatedAt string  `json:"createdAt"` // RFC3339
}

type statsActivity struct {
	Active24h      int `json:"active24h"`
	Active7d       int `json:"active7d"`
	ZeroWatchZones int `json:"zeroWatchZones"`
	NoEmail        int `json:"noEmail"`
}

type statsReach struct {
	WatchZones          int `json:"watchZones"`
	SavedApplications   int `json:"savedApplications"`
	DeviceRegistrations int `json:"deviceRegistrations"`
	NotificationsSent   int `json:"notificationsSent"`
	NotificationsUnread int `json:"notificationsUnread"`
}

// stats implements GET /v1/admin/stats: the whole-user-base aggregate. The user,
// signup and activity blocks come from the profiles UserStats SQL aggregate; the
// paying block is classified in Go from the paid-tier candidates via
// EffectiveTier (so the expiry/grace rule stays in the domain); the reach block
// sums the per-feature stores. Every reach store is optional — a nil (unwired)
// store contributes zero, never a 500.
func (h *handler) stats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := h.now()

	us, err := h.profiles.UserStats(ctx, now)
	if err != nil {
		h.serverError(w, r, "stats: user stats", err)
		return
	}

	candidates, err := h.profiles.PaidCandidates(ctx)
	if err != nil {
		h.serverError(w, r, "stats: paid candidates", err)
		return
	}

	savedTotal, err := h.savedTotal(ctx)
	if err != nil {
		h.serverError(w, r, "stats: saved total", err)
		return
	}
	deviceTotal, err := h.deviceTotal(ctx)
	if err != nil {
		h.serverError(w, r, "stats: device total", err)
		return
	}
	notifTotals, err := h.notifTotals(ctx)
	if err != nil {
		h.serverError(w, r, "stats: notification totals", err)
		return
	}

	resp := statsResponse{
		Users: statsUsers{
			Total: us.Total,
			ByTier: statsByTier{
				Free:     us.ByTier["Free"],
				Personal: us.ByTier["Personal"],
				Pro:      us.ByTier["Pro"],
			},
		},
		Paying: classifyPaying(candidates, now),
		Signups: statsSignups{
			Last24h:    us.Signups24h,
			Last7d:     us.Signups7d,
			Last30d:    us.Signups30d,
			MostRecent: mostRecent(us.MostRecent),
		},
		Activity: statsActivity{
			Active24h:      us.Active24h,
			Active7d:       us.Active7d,
			ZeroWatchZones: us.ZeroWatchZones,
			NoEmail:        us.NoEmail,
		},
		Reach: statsReach{
			WatchZones:          us.TotalWatchZones,
			SavedApplications:   savedTotal,
			DeviceRegistrations: deviceTotal,
			NotificationsSent:   notifTotals.Sent,
			NotificationsUnread: notifTotals.Unread,
		},
	}
	h.writeJSON(r, w, resp)
}

// classifyPaying buckets the paid-tier candidates by their EffectiveTier(now),
// mirroring the domain's lazy-expiry rule rather than the raw stored tier:
//   - effectivePaid: EffectiveTier(now) is still paid.
//   - appStore: effective-paid AND backed by an Apple original transaction id.
//   - appStoreByTier: appStore, additionally bucketed by EffectiveTier(now);
//     Personal+Pro always sums to appStore.
//   - comped: effective-paid with no original transaction id (offer/admin grant).
//   - lapsed: stored tier paid but EffectiveTier(now) has collapsed to Free.
//   - inGrace: effective-paid held alive ONLY by a live grace period (expiry
//     passed, grace end still ahead). It overlaps appStore/comped by design.
func classifyPaying(candidates []*profiles.UserProfile, now time.Time) statsPaying {
	var p statsPaying
	for _, c := range candidates {
		effective := c.EffectiveTier(now)
		switch {
		case effective.IsPaid():
			p.EffectivePaid++
			if c.OriginalTransactionID != nil {
				p.AppStore++
				switch effective {
				case profiles.TierPersonal:
					p.AppStoreByTier.Personal++
				case profiles.TierPro:
					p.AppStoreByTier.Pro++
				}
			} else {
				p.Comped++
			}
			if inGrace(c, now) {
				p.InGrace++
			}
		case c.Tier.IsPaid():
			// Stored paid but effective Free — the entitlement has lapsed.
			p.Lapsed++
		}
	}
	return p
}

// inGrace reports whether the profile is kept effective-paid solely by a live
// grace period: its subscription expiry has passed but its grace-period end is
// still ahead of now.
func inGrace(c *profiles.UserProfile, now time.Time) bool {
	return c.SubscriptionExpiry != nil && !c.SubscriptionExpiry.After(now) &&
		c.GracePeriodExpiry != nil && c.GracePeriodExpiry.After(now)
}

// mostRecent renders the profiles RecentSignup into the wire shape, preserving
// the null-when-empty contract.
func mostRecent(r *profiles.RecentSignup) *statsMostRecent {
	if r == nil {
		return nil
	}
	return &statsMostRecent{
		UserID:    r.UserID,
		Email:     r.Email,
		CreatedAt: r.CreatedAt.Format(time.RFC3339),
	}
}

// savedTotal returns the global saved-application count, or 0 when the store is
// unwired (store-less local boot).
func (h *handler) savedTotal(ctx context.Context) (int, error) {
	if h.savedCounts == nil {
		return 0, nil
	}
	return h.savedCounts.Count(ctx)
}

// deviceTotal returns the global device-registration count, or 0 when the store
// is unwired.
func (h *handler) deviceTotal(ctx context.Context) (int, error) {
	if h.deviceCounts == nil {
		return 0, nil
	}
	return h.deviceCounts.Count(ctx)
}

// notifTotals returns the whole-table notification tally, or the zero value when
// the store is unwired.
func (h *handler) notifTotals(ctx context.Context) (notifications.NotificationTotals, error) {
	if h.notifCounts == nil {
		return notifications.NotificationTotals{}, nil
	}
	return h.notifCounts.Totals(ctx)
}
