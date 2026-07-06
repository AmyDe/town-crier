package admin

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/offercodes"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// errBoom is a shared sentinel error for tests that just need "some failure".
var errBoom = errors.New("boom")

// fakeNotifCounts is a hand-written notificationCounts double: it records the
// user ids it was asked about and returns pre-configured per-user tallies and
// whole-table totals.
type fakeNotifCounts struct {
	counts map[string]notifications.NotificationCounts
	totals notifications.NotificationTotals
	gotIDs []string
}

func (f *fakeNotifCounts) CountsByUsers(_ context.Context, userIDs []string) (map[string]notifications.NotificationCounts, error) {
	f.gotIDs = userIDs
	return f.counts, nil
}

func (f *fakeNotifCounts) Totals(_ context.Context) (notifications.NotificationTotals, error) {
	return f.totals, nil
}

// fakeSavedCounts is a savedCountReader double: per-user saved counts plus a
// global total. It records the user ids it was asked about.
type fakeSavedCounts struct {
	counts map[string]int
	total  int
	gotIDs []string
}

func (f *fakeSavedCounts) CountsByUsers(_ context.Context, userIDs []string) (map[string]int, error) {
	f.gotIDs = userIDs
	return f.counts, nil
}

func (f *fakeSavedCounts) Count(_ context.Context) (int, error) { return f.total, nil }

// fakeDeviceCounts is a deviceCountReader double: per-user device counts plus a
// global total. It records the user ids it was asked about.
type fakeDeviceCounts struct {
	counts map[string]int
	total  int
	gotIDs []string
}

func (f *fakeDeviceCounts) CountsByUsers(_ context.Context, userIDs []string) (map[string]int, error) {
	f.gotIDs = userIDs
	return f.counts, nil
}

func (f *fakeDeviceCounts) Count(_ context.Context) (int, error) { return f.total, nil }

// fakeRedemptions is an offerRedemptionReader double: it returns pre-configured
// redemptions per user and records the user ids it was asked about.
type fakeRedemptions struct {
	byUser map[string][]offercodes.RedeemedOfferCode
	gotIDs []string
}

func (f *fakeRedemptions) RedeemedByUsers(_ context.Context, userIDs []string) (map[string][]offercodes.RedeemedOfferCode, error) {
	f.gotIDs = userIDs
	return f.byUser, nil
}

type fakeAdminStore struct {
	byEmail    map[string]*profiles.UserProfile
	saved      *profiles.UserProfile
	page       profiles.Page
	candidates []*profiles.UserProfile
	stats      profiles.UserStats

	gotSearch   string
	gotPageSize int
	gotToken    string
}

func (f *fakeAdminStore) PaidCandidates(_ context.Context) ([]*profiles.UserProfile, error) {
	return f.candidates, nil
}

func (f *fakeAdminStore) UserStats(_ context.Context, _ time.Time) (profiles.UserStats, error) {
	return f.stats, nil
}

func (f *fakeAdminStore) GetByEmail(_ context.Context, email string) (*profiles.UserProfile, error) {
	p, ok := f.byEmail[email]
	if !ok {
		return nil, profiles.ErrNotFound
	}
	return p, nil
}

func (f *fakeAdminStore) Save(_ context.Context, p *profiles.UserProfile) error {
	f.saved = p
	return nil
}

func (f *fakeAdminStore) List(_ context.Context, emailSearch string, pageSize int, continuationToken string) (profiles.Page, error) {
	f.gotSearch = emailSearch
	f.gotPageSize = pageSize
	f.gotToken = continuationToken
	return f.page, nil
}

type fakeTierSync struct {
	gotTier string
	calls   int
}

func (f *fakeTierSync) UpdateSubscriptionTier(_ context.Context, _, tier string) error {
	f.calls++
	f.gotTier = tier
	return nil
}

type fakeOfferStore struct {
	saved []offercodes.OfferCode

	listed        []offercodes.ListedOfferCode
	listErr       error
	gotLabel      *string
	gotLimit      int
	listCallCount int
}

func (f *fakeOfferStore) Save(_ context.Context, c offercodes.OfferCode) error {
	f.saved = append(f.saved, c)
	return nil
}

func (f *fakeOfferStore) List(_ context.Context, labelFilter *string, limit int) ([]offercodes.ListedOfferCode, error) {
	f.listCallCount++
	f.gotLabel = labelFilter
	f.gotLimit = limit
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listed, nil
}

type fakeGenerator struct {
	codes []string
	i     int
}

func (f *fakeGenerator) Generate() (string, error) {
	c := f.codes[f.i]
	f.i++
	return c, nil
}

func newTestHandler(store profileAdminStore, notifCounts notificationCounts, auth0 tierSync, codes offerCodeStore, gen codeGenerator, now time.Time) *handler {
	return &handler{
		profiles:    store,
		notifCounts: notifCounts,
		auth0:       auth0,
		codes:       codes,
		generator:   gen,
		now:         func() time.Time { return now },
		logger:      slog.New(slog.DiscardHandler),
	}
}

// activeCode builds a redemption whose window is still open at the test's
// clock, so RedeemedOfferCode.ActiveAt(now) is true.
func activeCode(code string, redeemedAt time.Time) offercodes.RedeemedOfferCode {
	at := redeemedAt
	return offercodes.RedeemedOfferCode{
		Code: code, Tier: profiles.TierPro, DurationDays: 30, RedeemedAt: &at,
	}
}

// expiredCode builds a redemption whose window has closed at the test's
// clock, so RedeemedOfferCode.ActiveAt(now) is false.
func expiredCode(code string, redeemedAt time.Time) offercodes.RedeemedOfferCode {
	at := redeemedAt
	return offercodes.RedeemedOfferCode{
		Code: code, Tier: profiles.TierPro, DurationDays: 30, RedeemedAt: &at,
	}
}

func strPtr(s string) *string { return &s }

func serve(t *testing.T, hf http.HandlerFunc, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *strings.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	} else {
		rdr = strings.NewReader("")
	}
	req := httptest.NewRequestWithContext(context.Background(), method, target, rdr)
	rec := httptest.NewRecorder()
	hf(rec, req)
	return rec
}

func freeProfile(t *testing.T) *profiles.UserProfile {
	t.Helper()
	p, err := profiles.NewProfile("auth0|u1", "u@example.com", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}
	return p
}

func TestGrant_ActivatesPaidTier(t *testing.T) {
	t.Parallel()

	store := &fakeAdminStore{byEmail: map[string]*profiles.UserProfile{"u@example.com": freeProfile(t)}}
	auth0 := &fakeTierSync{}
	h := newTestHandler(store, &fakeNotifCounts{}, auth0, &fakeOfferStore{}, &fakeGenerator{}, time.Now())

	rec := serve(t, h.grantSubscription, http.MethodPut, "/v1/admin/subscriptions", `{"email":"u@example.com","tier":"Pro"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != `{"userId":"auth0|u1","email":"u@example.com","tier":"Pro"}` {
		t.Errorf("body = %s", got)
	}
	if store.saved == nil || store.saved.Tier != profiles.TierPro || store.saved.SubscriptionExpiry == nil {
		t.Errorf("profile not activated: %+v", store.saved)
	}
	if !store.saved.SubscriptionExpiry.Equal(farFutureExpiry) {
		t.Errorf("expiry = %v, want far future", store.saved.SubscriptionExpiry)
	}
	if auth0.calls != 1 || auth0.gotTier != "Pro" {
		t.Errorf("auth0 sync = %+v", auth0)
	}
}

func TestGrant_FreeExpiresSubscription(t *testing.T) {
	t.Parallel()

	paid := freeProfile(t)
	paid.ActivateSubscription(profiles.TierPro, time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC))
	store := &fakeAdminStore{byEmail: map[string]*profiles.UserProfile{"u@example.com": paid}}
	h := newTestHandler(store, &fakeNotifCounts{}, &fakeTierSync{}, &fakeOfferStore{}, &fakeGenerator{}, time.Now())

	rec := serve(t, h.grantSubscription, http.MethodPut, "/v1/admin/subscriptions", `{"email":"u@example.com","tier":"Free"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Body.String(); got != `{"userId":"auth0|u1","email":"u@example.com","tier":"Free"}` {
		t.Errorf("body = %s", got)
	}
	if store.saved.Tier != profiles.TierFree || store.saved.SubscriptionExpiry != nil {
		t.Errorf("subscription not expired: %+v", store.saved)
	}
}

func TestGrant_EmailNotFound(t *testing.T) {
	t.Parallel()

	store := &fakeAdminStore{byEmail: map[string]*profiles.UserProfile{}}
	h := newTestHandler(store, &fakeNotifCounts{}, &fakeTierSync{}, &fakeOfferStore{}, &fakeGenerator{}, time.Now())

	rec := serve(t, h.grantSubscription, http.MethodPut, "/v1/admin/subscriptions", `{"email":"missing@example.com","tier":"Pro"}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("404 body = %q, want empty (backfilled downstream)", rec.Body.String())
	}
}

func TestGrant_InvalidTier(t *testing.T) {
	t.Parallel()

	store := &fakeAdminStore{byEmail: map[string]*profiles.UserProfile{"u@example.com": freeProfile(t)}}
	h := newTestHandler(store, &fakeNotifCounts{}, &fakeTierSync{}, &fakeOfferStore{}, &fakeGenerator{}, time.Now())

	rec := serve(t, h.grantSubscription, http.MethodPut, "/v1/admin/subscriptions", `{"email":"u@example.com","tier":"Bronze"}`)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestListUsers_ReturnsPage(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	// p1: watch-zone count, 2 unread of 57 notifications, 3 saved, 1 device, and
	// two redeemed offer codes — one expired, one still active.
	created1 := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	p1, _ := profiles.NewProfile("auth0|u1", "u@example.com", created1)
	wz := 2
	p1.WatchZoneCount = &wz
	// p2: legacy nil watch-zone count, no notifications, no saved/devices, and no
	// offer code (absent from every enrichment map).
	created2 := time.Date(2026, 2, 2, 10, 0, 0, 0, time.UTC)
	p2, _ := profiles.NewProfile("auth0|u2", "b@example.com", created2)
	p2.Tier = profiles.TierPro

	store := &fakeAdminStore{page: profiles.Page{Profiles: []*profiles.UserProfile{p1, p2}, ContinuationToken: "next"}}
	counts := &fakeNotifCounts{counts: map[string]notifications.NotificationCounts{
		"auth0|u1": {Total: 57, Unread: 2},
	}}
	saved := &fakeSavedCounts{counts: map[string]int{"auth0|u1": 3}}
	devices := &fakeDeviceCounts{counts: map[string]int{"auth0|u1": 1}}
	redemptions := &fakeRedemptions{byUser: map[string][]offercodes.RedeemedOfferCode{
		"auth0|u1": {
			expiredCode("OLDEXPIRED", now.Add(-40*24*time.Hour)),
			activeCode("NOWACTIVE", now.Add(-1*24*time.Hour)),
		},
	}}
	h := newTestHandler(store, counts, &fakeTierSync{}, &fakeOfferStore{}, &fakeGenerator{}, now)
	h.savedCounts = saved
	h.deviceCounts = devices
	h.redemptions = redemptions

	rec := serve(t, h.listUsers, http.MethodGet, "/v1/admin/users?search=example&pageSize=50", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	want := `{"items":[` +
		`{"userId":"auth0|u1","email":"u@example.com","tier":"Free","watchZoneCount":2,"createdAt":"2026-01-01T09:00:00Z","lastActiveAt":"2026-01-01T09:00:00Z","notificationTotal":57,"notificationUnread":2,"savedCount":3,"deviceCount":1,"offerCode":"NOWACTIVE"},` +
		`{"userId":"auth0|u2","email":"b@example.com","tier":"Pro","watchZoneCount":null,"createdAt":"2026-02-02T10:00:00Z","lastActiveAt":"2026-02-02T10:00:00Z","notificationTotal":0,"notificationUnread":0,"savedCount":0,"deviceCount":0,"offerCode":null}` +
		`],"continuationToken":"next"}`
	if got := rec.Body.String(); got != want {
		t.Errorf("body =\n  %s\nwant\n  %s", got, want)
	}
	if store.gotSearch != "example" || store.gotPageSize != 50 {
		t.Errorf("forwarded search=%q pageSize=%d", store.gotSearch, store.gotPageSize)
	}
	// Each enrichment batches the whole page's user ids into a single lookup.
	for name, got := range map[string][]string{
		"notifs":      counts.gotIDs,
		"saved":       saved.gotIDs,
		"devices":     devices.gotIDs,
		"redemptions": redemptions.gotIDs,
	} {
		if len(got) != 2 || got[0] != "auth0|u1" || got[1] != "auth0|u2" {
			t.Errorf("%s lookup ids = %v, want [auth0|u1 auth0|u2]", name, got)
		}
	}
}

// TestListUsers_NilEnrichmentStores skips every enrichment when its store is
// unwired (store-less local boot): the row still renders with zero counts and a
// null offer code, never a panic or 500.
func TestListUsers_NilEnrichmentStores(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	p, _ := profiles.NewProfile("auth0|u1", "u@example.com", created)
	store := &fakeAdminStore{page: profiles.Page{Profiles: []*profiles.UserProfile{p}}}
	// notifCounts nil too — every enrichment store is unwired.
	h := newTestHandler(store, nil, &fakeTierSync{}, &fakeOfferStore{}, &fakeGenerator{}, time.Now())

	rec := serve(t, h.listUsers, http.MethodGet, "/v1/admin/users", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	want := `{"items":[` +
		`{"userId":"auth0|u1","email":"u@example.com","tier":"Free","watchZoneCount":null,"createdAt":"2026-01-01T09:00:00Z","lastActiveAt":"2026-01-01T09:00:00Z","notificationTotal":0,"notificationUnread":0,"savedCount":0,"deviceCount":0,"offerCode":null}` +
		`],"continuationToken":null}`
	if got := rec.Body.String(); got != want {
		t.Errorf("body =\n  %s\nwant\n  %s", got, want)
	}
}

func TestListUsers_DefaultsAndNullToken(t *testing.T) {
	t.Parallel()

	store := &fakeAdminStore{page: profiles.Page{Profiles: nil, ContinuationToken: ""}}
	counts := &fakeNotifCounts{}
	h := newTestHandler(store, counts, &fakeTierSync{}, &fakeOfferStore{}, &fakeGenerator{}, time.Now())

	rec := serve(t, h.listUsers, http.MethodGet, "/v1/admin/users", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Body.String(); got != `{"items":[],"continuationToken":null}` {
		t.Errorf("body = %s", got)
	}
	if store.gotPageSize != 20 {
		t.Errorf("default pageSize = %d, want 20", store.gotPageSize)
	}
	// An empty page must not trigger a counts lookup at all.
	if counts.gotIDs != nil {
		t.Errorf("empty page issued a counts lookup for %v", counts.gotIDs)
	}
}

func TestListUsers_InvalidPageSize(t *testing.T) {
	t.Parallel()

	h := newTestHandler(&fakeAdminStore{}, &fakeNotifCounts{}, &fakeTierSync{}, &fakeOfferStore{}, &fakeGenerator{}, time.Now())
	rec := serve(t, h.listUsers, http.MethodGet, "/v1/admin/users?pageSize=abc", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestGenerate_HappyPath(t *testing.T) {
	t.Parallel()

	offerStore := &fakeOfferStore{}
	gen := &fakeGenerator{codes: []string{"ABCDEFGHJKMN", "NPQRSTVWXYZ0"}}
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	h := newTestHandler(&fakeAdminStore{}, &fakeNotifCounts{}, &fakeTierSync{}, offerStore, gen, now)

	rec := serve(t, h.generateOfferCodes, http.MethodPost, "/v1/admin/offer-codes",
		`{"count":2,"tier":"Pro","durationDays":30,"label":"creator-campaign","maxRedemptions":5}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain" {
		t.Errorf("content-type = %q", ct)
	}
	if got := rec.Body.String(); got != "ABCD-EFGH-JKMN\nNPQR-STVW-XYZ0\n" {
		t.Errorf("body = %q", got)
	}
	// The stored codes are canonical (un-hyphenated), carry the label and
	// redemption cap from the request, for the granted tier.
	if len(offerStore.saved) != 2 || offerStore.saved[0].Code != "ABCDEFGHJKMN" || offerStore.saved[0].Tier != profiles.TierPro {
		t.Errorf("saved = %+v", offerStore.saved)
	}
	if offerStore.saved[0].Label != "creator-campaign" || offerStore.saved[0].MaxRedemptions != 5 {
		t.Errorf("saved label/maxRedemptions = %+v", offerStore.saved[0])
	}
}

// TestGenerate_DefaultsMaxRedemptionsToOne confirms that omitting
// maxRedemptions mints a single-use code (maxRedemptions=1) — the pre-existing
// behaviour every code used to have.
func TestGenerate_DefaultsMaxRedemptionsToOne(t *testing.T) {
	t.Parallel()

	offerStore := &fakeOfferStore{}
	gen := &fakeGenerator{codes: []string{"ABCDEFGHJKMN"}}
	h := newTestHandler(&fakeAdminStore{}, &fakeNotifCounts{}, &fakeTierSync{}, offerStore, gen, time.Now())

	rec := serve(t, h.generateOfferCodes, http.MethodPost, "/v1/admin/offer-codes",
		`{"count":1,"tier":"Pro","durationDays":30,"label":"single-use"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if len(offerStore.saved) != 1 || offerStore.saved[0].MaxRedemptions != 1 {
		t.Errorf("saved = %+v, want MaxRedemptions=1", offerStore.saved)
	}
}

func TestGenerate_ValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{"count too low", `{"count":0,"tier":"Pro","durationDays":30,"label":"l"}`, `{"error":"Count must be between 1 and 1000. (Parameter 'command')\nActual value was 0.","message":null}`},
		{"count too high", `{"count":2000,"tier":"Pro","durationDays":30,"label":"l"}`, `{"error":"Count must be between 1 and 1000. (Parameter 'command')\nActual value was 2000.","message":null}`},
		{"free tier", `{"count":1,"tier":"Free","durationDays":30,"label":"l"}`, `{"error":"Offer codes cannot grant the Free tier. (Parameter 'command')","message":null}`},
		{"duration too high", `{"count":1,"tier":"Pro","durationDays":400,"label":"l"}`, `{"error":"DurationDays must be between 1 and 365. (Parameter 'command')\nActual value was 400.","message":null}`},
		{"label missing", `{"count":1,"tier":"Pro","durationDays":30}`, `{"error":"Label is required. (Parameter 'command')","message":null}`},
		{"label blank", `{"count":1,"tier":"Pro","durationDays":30,"label":"   "}`, `{"error":"Label is required. (Parameter 'command')","message":null}`},
		{"maxRedemptions zero", `{"count":1,"tier":"Pro","durationDays":30,"label":"l","maxRedemptions":0}`, `{"error":"MaxRedemptions must be between 1 and 10000. (Parameter 'command')\nActual value was 0.","message":null}`},
		{"maxRedemptions too high", `{"count":1,"tier":"Pro","durationDays":30,"label":"l","maxRedemptions":10001}`, `{"error":"MaxRedemptions must be between 1 and 10000. (Parameter 'command')\nActual value was 10001.","message":null}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newTestHandler(&fakeAdminStore{}, &fakeNotifCounts{}, &fakeTierSync{}, &fakeOfferStore{}, &fakeGenerator{}, time.Now())
			rec := serve(t, h.generateOfferCodes, http.MethodPost, "/v1/admin/offer-codes", tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			if got := rec.Body.String(); got != tc.want {
				t.Errorf("body = %q\nwant %q", got, tc.want)
			}
		})
	}
}

// paidCandidate builds a paid-tier UserProfile for the stats classification test.
func paidCandidate(userID string, tier profiles.SubscriptionTier, expiry *time.Time, grace *time.Time, otid *string) *profiles.UserProfile {
	return &profiles.UserProfile{
		UserID:                userID,
		Tier:                  tier,
		SubscriptionExpiry:    expiry,
		GracePeriodExpiry:     grace,
		OriginalTransactionID: otid,
	}
}

// TestStats_ReturnsPinnedContract exercises the full aggregate: the paying
// buckets classified via EffectiveTier (an App Store, a comped, a lapsed and an
// in-grace candidate), the user/tier/signup/activity blocks, and the reach
// totals — asserting the exact pinned JSON contract the CLI mirror depends on.
func TestStats_ReturnsPinnedContract(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	future := now.Add(30 * 24 * time.Hour)
	past := now.Add(-1 * time.Hour)
	txn := "1000000000000001"

	recent := time.Date(2026, 6, 30, 9, 0, 0, 0, time.UTC)
	recentEmail := "new@example.com"
	store := &fakeAdminStore{
		stats: profiles.UserStats{
			Total:           100,
			ByTier:          map[string]int{"Free": 70, "Personal": 20, "Pro": 10},
			Signups24h:      5,
			Signups7d:       12,
			Signups30d:      30,
			MostRecent:      &profiles.RecentSignup{UserID: "auth0|new", Email: &recentEmail, CreatedAt: recent},
			Active24h:       8,
			Active7d:        20,
			ZeroWatchZones:  15,
			NoEmail:         3,
			TotalWatchZones: 250,
		},
		candidates: []*profiles.UserProfile{
			paidCandidate("auth0|appstore", profiles.TierPro, &future, nil, &txn),   // effective-paid + appStore
			paidCandidate("auth0|comped", profiles.TierPersonal, &future, nil, nil), // effective-paid + comped
			paidCandidate("auth0|lapsed", profiles.TierPro, &past, nil, nil),        // lapsed (expired, no grace)
			paidCandidate("auth0|grace", profiles.TierPro, &past, &future, nil),     // effective-paid via live grace + comped + inGrace
		},
	}
	counts := &fakeNotifCounts{totals: notifications.NotificationTotals{Sent: 9000, Unread: 1200}}
	h := newTestHandler(store, counts, &fakeTierSync{}, &fakeOfferStore{}, &fakeGenerator{}, now)
	h.savedCounts = &fakeSavedCounts{total: 500}
	h.deviceCounts = &fakeDeviceCounts{total: 300}
	h.redemptions = &fakeRedemptions{}

	rec := serve(t, h.stats, http.MethodGet, "/v1/admin/stats", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	want := `{` +
		`"users":{"total":100,"byTier":{"Free":70,"Personal":20,"Pro":10}},` +
		`"paying":{"effectivePaid":3,"appStore":1,"comped":2,"lapsed":1,"inGrace":1},` +
		`"signups":{"last24h":5,"last7d":12,"last30d":30,"mostRecent":{"userId":"auth0|new","email":"new@example.com","createdAt":"2026-06-30T09:00:00Z"}},` +
		`"activity":{"active24h":8,"active7d":20,"zeroWatchZones":15,"noEmail":3},` +
		`"reach":{"watchZones":250,"savedApplications":500,"deviceRegistrations":300,"notificationsSent":9000,"notificationsUnread":1200}` +
		`}`
	if got := rec.Body.String(); got != want {
		t.Errorf("body =\n  %s\nwant\n  %s", got, want)
	}
}

// TestStats_NoUsers_NullMostRecentAndNilStores renders the empty base with a
// null mostRecent and skips the reach stores when unwired (nil), never panicking.
func TestStats_NoUsers_NullMostRecentAndNilStores(t *testing.T) {
	t.Parallel()

	store := &fakeAdminStore{stats: profiles.UserStats{}} // Total 0, MostRecent nil, ByTier nil
	// notifCounts nil and saved/device readers left unset (nil): the reach block
	// must fall back to zeros without a nil dereference.
	h := newTestHandler(store, nil, &fakeTierSync{}, &fakeOfferStore{}, &fakeGenerator{}, time.Now())

	rec := serve(t, h.stats, http.MethodGet, "/v1/admin/stats", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	want := `{` +
		`"users":{"total":0,"byTier":{"Free":0,"Personal":0,"Pro":0}},` +
		`"paying":{"effectivePaid":0,"appStore":0,"comped":0,"lapsed":0,"inGrace":0},` +
		`"signups":{"last24h":0,"last7d":0,"last30d":0,"mostRecent":null},` +
		`"activity":{"active24h":0,"active7d":0,"zeroWatchZones":0,"noEmail":0},` +
		`"reach":{"watchZones":0,"savedApplications":0,"deviceRegistrations":0,"notificationsSent":0,"notificationsUnread":0}` +
		`}`
	if got := rec.Body.String(); got != want {
		t.Errorf("body =\n  %s\nwant\n  %s", got, want)
	}
}

// ---------------------------------------------------------------------------
// GET /v1/admin/offer-codes (listOfferCodes)
// ---------------------------------------------------------------------------

func TestListOfferCodes_ReturnsPinnedShape(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	lastRedeemed := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	offerStore := &fakeOfferStore{listed: []offercodes.ListedOfferCode{
		{
			OfferCode: offercodes.OfferCode{
				Code: "ABCDEFGHJKMN", Tier: profiles.TierPro, DurationDays: 30,
				CreatedAt: created, Label: "creator-campaign", MaxRedemptions: 3, RedemptionCount: 2,
			},
			LastRedeemedAt: &lastRedeemed,
		},
		{
			OfferCode: offercodes.OfferCode{
				Code: "NPQRSTVWXYZ0", Tier: profiles.TierPersonal, DurationDays: 7,
				CreatedAt: created, Label: "unused", MaxRedemptions: 1, RedemptionCount: 0,
			},
			LastRedeemedAt: nil,
		},
	}}
	h := newTestHandler(&fakeAdminStore{}, &fakeNotifCounts{}, &fakeTierSync{}, offerStore, &fakeGenerator{}, time.Now())

	rec := serve(t, h.listOfferCodes, http.MethodGet, "/v1/admin/offer-codes", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	want := `[` +
		`{"code":"ABCD-EFGH-JKMN","label":"creator-campaign","tier":"Pro","durationDays":30,"maxRedemptions":3,"redemptionCount":2,"createdAt":"2026-06-01T09:00:00Z","lastRedeemedAt":"2026-06-15T10:00:00Z"},` +
		`{"code":"NPQR-STVW-XYZ0","label":"unused","tier":"Personal","durationDays":7,"maxRedemptions":1,"redemptionCount":0,"createdAt":"2026-06-01T09:00:00Z","lastRedeemedAt":null}` +
		`]`
	if got := rec.Body.String(); got != want {
		t.Errorf("body =\n  %s\nwant\n  %s", got, want)
	}
	// Default limit applies when the query omits ?limit.
	if offerStore.gotLimit != defaultListLimit {
		t.Errorf("limit = %d, want default %d", offerStore.gotLimit, defaultListLimit)
	}
	if offerStore.gotLabel != nil {
		t.Errorf("label filter = %v, want nil (no ?label given)", offerStore.gotLabel)
	}
}

func TestListOfferCodes_FiltersByLabelAndLimit(t *testing.T) {
	t.Parallel()

	offerStore := &fakeOfferStore{listed: []offercodes.ListedOfferCode{}}
	h := newTestHandler(&fakeAdminStore{}, &fakeNotifCounts{}, &fakeTierSync{}, offerStore, &fakeGenerator{}, time.Now())

	rec := serve(t, h.listOfferCodes, http.MethodGet, "/v1/admin/offer-codes?label=creator&limit=10", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != "[]" {
		t.Errorf("body = %q, want []", got)
	}
	if offerStore.gotLabel == nil || *offerStore.gotLabel != "creator" {
		t.Errorf("label filter = %v, want \"creator\"", offerStore.gotLabel)
	}
	if offerStore.gotLimit != 10 {
		t.Errorf("limit = %d, want 10", offerStore.gotLimit)
	}
}

func TestListOfferCodes_InvalidLimit(t *testing.T) {
	t.Parallel()

	h := newTestHandler(&fakeAdminStore{}, &fakeNotifCounts{}, &fakeTierSync{}, &fakeOfferStore{}, &fakeGenerator{}, time.Now())
	rec := serve(t, h.listOfferCodes, http.MethodGet, "/v1/admin/offer-codes?limit=abc", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestListOfferCodes_StoreError(t *testing.T) {
	t.Parallel()

	offerStore := &fakeOfferStore{listErr: errBoom}
	h := newTestHandler(&fakeAdminStore{}, &fakeNotifCounts{}, &fakeTierSync{}, offerStore, &fakeGenerator{}, time.Now())
	rec := serve(t, h.listOfferCodes, http.MethodGet, "/v1/admin/offer-codes", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}
