package admin

import (
	"net/http"
	"strconv"
)

// defaultPageSize mirrors the .NET admin endpoint default (pageSize ?? 20).
const defaultPageSize = 20

type listItem struct {
	UserID string  `json:"userId"`
	Email  *string `json:"email"`
	Tier   string  `json:"tier"`
}

// listResult mirrors the .NET ListUsersResult: { items, continuationToken }.
// continuationToken is null when the query is exhausted.
type listResult struct {
	Items             []listItem `json:"items"`
	ContinuationToken *string    `json:"continuationToken"`
}

// listUsers implements GET /v1/admin/users?search&pageSize&continuationToken: a
// cross-partition page of profiles, optionally filtered by case-insensitive
// email substring. An unparseable pageSize is a bodyless 400 (the .NET int?
// binding failure).
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

	items := make([]listItem, 0, len(page.Profiles))
	for _, p := range page.Profiles {
		items = append(items, listItem{UserID: p.UserID, Email: p.Email, Tier: p.Tier.String()})
	}
	var token *string
	if page.ContinuationToken != "" {
		t := page.ContinuationToken
		token = &t
	}
	h.writeJSON(r, w, listResult{Items: items, ContinuationToken: token})
}
