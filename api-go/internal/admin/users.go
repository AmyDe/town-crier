package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/notifications"
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

	// Enrich each row with its notification tally in a single batched lookup
	// after List returns, so the profile query stays purely about profiles.
	// Users absent from the tally default to {0, 0} via the map zero value.
	counts, err := h.notificationCountsFor(r, page)
	if err != nil {
		h.serverError(w, r, "list users: counts", err)
		return
	}

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
	ids := make([]string, 0, len(page.Profiles))
	for _, p := range page.Profiles {
		ids = append(ids, p.UserID)
	}
	return h.notifCounts.CountsByUsers(r.Context(), ids)
}
