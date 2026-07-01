package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/offercodes"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// defaultPageSize is the default page size for the admin users list.
const defaultPageSize = 20

type listItem struct {
	UserID             string  `json:"userId"`
	Email              *string `json:"email"`
	Tier               string  `json:"tier"`
	WatchZoneCount     *int    `json:"watchZoneCount"`
	CreatedAt          string  `json:"createdAt"`    // RFC3339
	LastActiveAt       string  `json:"lastActiveAt"` // RFC3339
	NotificationTotal  int     `json:"notificationTotal"`
	NotificationUnread int     `json:"notificationUnread"`
	SavedCount         int     `json:"savedCount"`
	DeviceCount        int     `json:"deviceCount"`
	// OfferCode is the user's currently-active offer code (still within its
	// redeemed_at + duration window), or null when none is active.
	OfferCode *string `json:"offerCode"`
}

// listResult is the response shape for GET /v1/admin/users: { items, continuationToken }.
// continuationToken is null when the query is exhausted.
type listResult struct {
	Items             []listItem `json:"items"`
	ContinuationToken *string    `json:"continuationToken"`
}

// listUsers implements GET /v1/admin/users?search&pageSize&continuationToken: a
// cross-partition page of profiles, optionally filtered by case-insensitive
// email substring. An unparseable pageSize is a bodyless 400.
func (h *handler) listUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	search := q.Get("search")
	pageSize := defaultPageSize
	if raw := q.Get("pageSize"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		pageSize = n
	}
	continuationToken := q.Get("continuationToken")

	page, err := h.profiles.List(r.Context(), search, pageSize, continuationToken)
	if err != nil {
		h.serverError(w, r, "list users", err)
		return
	}

	// Enrich each row with its per-user tallies in one batched lookup per column
	// after List returns, so the profile query stays purely about profiles. Each
	// enrichment is keyed on the page's user ids and skipped when its store is
	// unwired; users absent from a tally default to the map zero value.
	counts, err := h.notificationCountsFor(r, page)
	if err != nil {
		h.serverError(w, r, "list users: counts", err)
		return
	}
	savedCounts, err := h.savedCountsFor(r, page)
	if err != nil {
		h.serverError(w, r, "list users: saved counts", err)
		return
	}
	deviceCounts, err := h.deviceCountsFor(r, page)
	if err != nil {
		h.serverError(w, r, "list users: device counts", err)
		return
	}
	redemptions, err := h.redemptionsFor(r, page)
	if err != nil {
		h.serverError(w, r, "list users: redemptions", err)
		return
	}
	now := h.now()

	items := make([]listItem, 0, len(page.Profiles))
	for _, p := range page.Profiles {
		nc := counts[p.UserID]
		items = append(items, listItem{
			UserID:             p.UserID,
			Email:              p.Email,
			Tier:               p.Tier.String(),
			WatchZoneCount:     p.WatchZoneCount,
			CreatedAt:          p.CreatedAt.Format(time.RFC3339),
			LastActiveAt:       p.LastActiveAt.Format(time.RFC3339),
			NotificationTotal:  nc.Total,
			NotificationUnread: nc.Unread,
			SavedCount:         savedCounts[p.UserID],
			DeviceCount:        deviceCounts[p.UserID],
			OfferCode:          activeOfferCode(redemptions[p.UserID], now),
		})
	}
	var token *string
	if page.ContinuationToken != "" {
		t := page.ContinuationToken
		token = &t
	}
	h.writeJSON(r, w, listResult{Items: items, ContinuationToken: token})
}

// notificationCountsFor returns the per-user notification tally for the page in
// a single batched lookup. An empty page (or an unwired counts store) skips the
// query and returns an empty map, leaving every row's counts at {0, 0}.
func (h *handler) notificationCountsFor(r *http.Request, page profiles.Page) (map[string]notifications.NotificationCounts, error) {
	if h.notifCounts == nil || len(page.Profiles) == 0 {
		return map[string]notifications.NotificationCounts{}, nil
	}
	return h.notifCounts.CountsByUsers(r.Context(), pageUserIDs(page))
}

// savedCountsFor returns the per-user saved-application count for the page in one
// batched lookup, skipping the query when the store is unwired or the page empty.
func (h *handler) savedCountsFor(r *http.Request, page profiles.Page) (map[string]int, error) {
	if h.savedCounts == nil || len(page.Profiles) == 0 {
		return map[string]int{}, nil
	}
	return h.savedCounts.CountsByUsers(r.Context(), pageUserIDs(page))
}

// deviceCountsFor returns the per-user device-registration count for the page in
// one batched lookup, skipping the query when the store is unwired or page empty.
func (h *handler) deviceCountsFor(r *http.Request, page profiles.Page) (map[string]int, error) {
	if h.deviceCounts == nil || len(page.Profiles) == 0 {
		return map[string]int{}, nil
	}
	return h.deviceCounts.CountsByUsers(r.Context(), pageUserIDs(page))
}

// redemptionsFor returns each user's redeemed offer codes for the page in one
// batched lookup, skipping the query when the store is unwired or the page empty.
func (h *handler) redemptionsFor(r *http.Request, page profiles.Page) (map[string][]offercodes.OfferCode, error) {
	if h.redemptions == nil || len(page.Profiles) == 0 {
		return map[string][]offercodes.OfferCode{}, nil
	}
	return h.redemptions.RedeemedByUsers(r.Context(), pageUserIDs(page))
}

// pageUserIDs collects the page's user ids for a batched enrichment lookup.
func pageUserIDs(page profiles.Page) []string {
	ids := make([]string, 0, len(page.Profiles))
	for _, p := range page.Profiles {
		ids = append(ids, p.UserID)
	}
	return ids
}

// activeOfferCode returns the first still-active code (its redeemed_at + duration
// window has not closed at now) among the user's redeemed codes, or nil when none
// is active. A user can hold several redeemed codes; only a live one is surfaced.
func activeOfferCode(codes []offercodes.OfferCode, now time.Time) *string {
	for i := range codes {
		if codes[i].ActiveAt(now) {
			code := codes[i].Code
			return &code
		}
	}
	return nil
}
