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
// MaxRedemptions is a pointer with omitempty so an unset --max-redemptions
// flag omits the field entirely, letting the API default it to 1 — an
// explicit 0 (impossible here since parseStrictInt rejects it before this
// struct is built) would otherwise be indistinguishable from "not given".
type generateOfferCodesRequest struct {
	Count          int    `json:"count"`
	Tier           string `json:"tier"`
	DurationDays   int    `json:"durationDays"`
	Label          string `json:"label"`
	MaxRedemptions *int   `json:"maxRedemptions,omitempty"`
}

// listOfferCodesResponse is the GET /v1/admin/offer-codes response body: a
// bare array, newest-first, no pagination beyond the API's own limit.
type listOfferCodesResponse []offerCodeListItem

type offerCodeListItem struct {
	Code            string  `json:"code"`
	Label           string  `json:"label"`
	Tier            string  `json:"tier"`
	DurationDays    int     `json:"durationDays"`
	MaxRedemptions  int     `json:"maxRedemptions"`
	RedemptionCount int     `json:"redemptionCount"`
	CreatedAt       string  `json:"createdAt"`
	LastRedeemedAt  *string `json:"lastRedeemedAt"`
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
	UserID             string  `json:"userId"`
	Email              *string `json:"email"`
	Tier               string  `json:"tier"`
	WatchZoneCount     *int    `json:"watchZoneCount"`
	CreatedAt          *string `json:"createdAt"`
	LastActiveAt       *string `json:"lastActiveAt"`
	NotificationTotal  int     `json:"notificationTotal"`
	NotificationUnread int     `json:"notificationUnread"`
	SavedCount         int     `json:"savedCount"`
	DeviceCount        int     `json:"deviceCount"`
	OfferCode          *string `json:"offerCode"`
}

// statsResponse mirrors the pinned GET /v1/admin/stats JSON contract
// byte-for-field (api-go/internal/admin/stats.go). Field names and nesting are
// load-bearing: the API side is authoritative and this type must not drift.
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
// render in a fixed order, matching the API's encoding.
type statsByTier struct {
	Free     int `json:"Free"`
	Personal int `json:"Personal"`
	Pro      int `json:"Pro"`
}

type statsPaying struct {
	EffectivePaid int `json:"effectivePaid"`
	AppStore      int `json:"appStore"`
	Comped        int `json:"comped"`
	Lapsed        int `json:"lapsed"`
	InGrace       int `json:"inGrace"`
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
	CreatedAt string  `json:"createdAt"`
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
