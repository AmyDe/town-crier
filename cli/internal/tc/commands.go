package tc

import (
	"strconv"
	"strings"
)

// Exit codes shared across commands:
//
//	0 success, 1 usage/validation/config error, 2 API/runtime error.
const (
	exitOK      = 0
	exitUsage   = 1
	exitRuntime = 2
)

// generateOfferCodesRequest is the POST /v1/admin/offer-codes body.
type generateOfferCodesRequest struct {
	Count        int    `json:"count"`
	Tier         string `json:"tier"`
	DurationDays int    `json:"durationDays"`
}

// grantSubscriptionRequest is the PUT /v1/admin/subscriptions body.
type grantSubscriptionRequest struct {
	Email string `json:"email"`
	Tier  string `json:"tier"`
}

// listUsersResponse is the GET /v1/admin/users response body. continuationToken
// is null when the query is exhausted.
type listUsersResponse struct {
	Items             []listUsersItem `json:"items"`
	ContinuationToken *string         `json:"continuationToken"`
}

type listUsersItem struct {
	UserID string  `json:"userId"`
	Email  *string `json:"email"`
	Tier   string  `json:"tier"`
}

// parseStrictInt parses s as a non-negative base-10 integer, accepting only
// ASCII digits. It rejects signs, whitespace, and decimals. Overflow is
// reported as invalid.
func parseStrictInt(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}

// normalizeTier matches tier case-insensitively against the allowed set and
// returns the canonical casing the API expects.
func normalizeTier(tier string, valid []string) (string, bool) {
	for _, v := range valid {
		if strings.EqualFold(tier, v) {
			return v, true
		}
	}
	return "", false
}
