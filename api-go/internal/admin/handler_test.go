package admin

import (
	"context"
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

// fakeNotifCounts is a hand-written notificationCounts double: it records the
// user ids it was asked about and returns pre-configured tallies.
type fakeNotifCounts struct {
	counts map[string]notifications.NotificationCounts
	gotIDs []string
}

func (f *fakeNotifCounts) CountsByUsers(_ context.Context, userIDs []string) (map[string]notifications.NotificationCounts, error) {
	f.gotIDs = userIDs
	return f.counts, nil
}

type fakeAdminStore struct {
	byEmail map[string]*profiles.UserProfile
	saved   *profiles.UserProfile
	page    profiles.Page

	gotSearch   string
	gotPageSize int
	gotToken    string
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
}

func (f *fakeOfferStore) Save(_ context.Context, c offercodes.OfferCode) error {
	f.saved = append(f.saved, c)
	return nil
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

	// p1: has a watch-zone count and 2 unread of 57 notifications.
	created1 := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	p1, _ := profiles.NewProfile("auth0|u1", "u@example.com", created1)
	wz := 2
	p1.WatchZoneCount = &wz
	// p2: legacy nil watch-zone count and no notifications (absent from counts).
	created2 := time.Date(2026, 2, 2, 10, 0, 0, 0, time.UTC)
	p2, _ := profiles.NewProfile("auth0|u2", "b@example.com", created2)
	p2.Tier = profiles.TierPro

	store := &fakeAdminStore{page: profiles.Page{Profiles: []*profiles.UserProfile{p1, p2}, ContinuationToken: "next"}}
	counts := &fakeNotifCounts{counts: map[string]notifications.NotificationCounts{
		"auth0|u1": {Total: 57, Unread: 2},
	}}
	h := newTestHandler(store, counts, &fakeTierSync{}, &fakeOfferStore{}, &fakeGenerator{}, time.Now())

	rec := serve(t, h.listUsers, http.MethodGet, "/v1/admin/users?search=example&pageSize=50", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	want := `{"items":[` +
		`{"userId":"auth0|u1","email":"u@example.com","tier":"Free","watchZoneCount":2,"createdAt":"2026-01-01T09:00:00Z","lastActiveAt":"2026-01-01T09:00:00Z","notificationTotal":57,"notificationUnread":2},` +
		`{"userId":"auth0|u2","email":"b@example.com","tier":"Pro","watchZoneCount":null,"createdAt":"2026-02-02T10:00:00Z","lastActiveAt":"2026-02-02T10:00:00Z","notificationTotal":0,"notificationUnread":0}` +
		`],"continuationToken":"next"}`
	if got := rec.Body.String(); got != want {
		t.Errorf("body =\n  %s\nwant\n  %s", got, want)
	}
	if store.gotSearch != "example" || store.gotPageSize != 50 {
		t.Errorf("forwarded search=%q pageSize=%d", store.gotSearch, store.gotPageSize)
	}
	// The handler batches the whole page's user ids into a single counts lookup.
	if len(counts.gotIDs) != 2 || counts.gotIDs[0] != "auth0|u1" || counts.gotIDs[1] != "auth0|u2" {
		t.Errorf("counts lookup ids = %v, want [auth0|u1 auth0|u2]", counts.gotIDs)
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

	rec := serve(t, h.generateOfferCodes, http.MethodPost, "/v1/admin/offer-codes", `{"count":2,"tier":"Pro","durationDays":30}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain" {
		t.Errorf("content-type = %q", ct)
	}
	if got := rec.Body.String(); got != "ABCD-EFGH-JKMN\nNPQR-STVW-XYZ0\n" {
		t.Errorf("body = %q", got)
	}
	// The stored codes are canonical (un-hyphenated) for the granted tier.
	if len(offerStore.saved) != 2 || offerStore.saved[0].Code != "ABCDEFGHJKMN" || offerStore.saved[0].Tier != profiles.TierPro {
		t.Errorf("saved = %+v", offerStore.saved)
	}
}

func TestGenerate_ValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{"count too low", `{"count":0,"tier":"Pro","durationDays":30}`, `{"error":"Count must be between 1 and 1000. (Parameter 'command')\nActual value was 0.","message":null}`},
		{"count too high", `{"count":2000,"tier":"Pro","durationDays":30}`, `{"error":"Count must be between 1 and 1000. (Parameter 'command')\nActual value was 2000.","message":null}`},
		{"free tier", `{"count":1,"tier":"Free","durationDays":30}`, `{"error":"Offer codes cannot grant the Free tier. (Parameter 'command')","message":null}`},
		{"duration too high", `{"count":1,"tier":"Pro","durationDays":400}`, `{"error":"DurationDays must be between 1 and 365. (Parameter 'command')\nActual value was 400.","message":null}`},
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
