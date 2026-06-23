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

	"github.com/AmyDe/town-crier/api-go/internal/offercodes"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

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

type fakeBackfiller struct {
	result watchzones.BackfillResult
	err    error
	calls  int
}

func (f *fakeBackfiller) BackfillLocation(_ context.Context) (watchzones.BackfillResult, error) {
	f.calls++
	if f.err != nil {
		return watchzones.BackfillResult{}, f.err
	}
	return f.result, nil
}

func newTestHandler(store profileAdminStore, auth0 tierSync, codes offerCodeStore, gen codeGenerator, now time.Time) *handler {
	return &handler{
		profiles:   store,
		auth0:      auth0,
		codes:      codes,
		generator:  gen,
		watchZones: &fakeBackfiller{},
		now:        func() time.Time { return now },
		logger:     slog.New(slog.DiscardHandler),
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
	h := newTestHandler(store, auth0, &fakeOfferStore{}, &fakeGenerator{}, time.Now())

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
	h := newTestHandler(store, &fakeTierSync{}, &fakeOfferStore{}, &fakeGenerator{}, time.Now())

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
	h := newTestHandler(store, &fakeTierSync{}, &fakeOfferStore{}, &fakeGenerator{}, time.Now())

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
	h := newTestHandler(store, &fakeTierSync{}, &fakeOfferStore{}, &fakeGenerator{}, time.Now())

	rec := serve(t, h.grantSubscription, http.MethodPut, "/v1/admin/subscriptions", `{"email":"u@example.com","tier":"Bronze"}`)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestListUsers_ReturnsPage(t *testing.T) {
	t.Parallel()

	p1 := freeProfile(t)
	p2, _ := profiles.NewProfile("auth0|u2", "b@example.com", time.Now())
	p2.Tier = profiles.TierPro
	store := &fakeAdminStore{page: profiles.Page{Profiles: []*profiles.UserProfile{p1, p2}, ContinuationToken: "next"}}
	h := newTestHandler(store, &fakeTierSync{}, &fakeOfferStore{}, &fakeGenerator{}, time.Now())

	rec := serve(t, h.listUsers, http.MethodGet, "/v1/admin/users?search=example&pageSize=50", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	want := `{"items":[{"userId":"auth0|u1","email":"u@example.com","tier":"Free"},{"userId":"auth0|u2","email":"b@example.com","tier":"Pro"}],"continuationToken":"next"}`
	if got := rec.Body.String(); got != want {
		t.Errorf("body =\n  %s\nwant\n  %s", got, want)
	}
	if store.gotSearch != "example" || store.gotPageSize != 50 {
		t.Errorf("forwarded search=%q pageSize=%d", store.gotSearch, store.gotPageSize)
	}
}

func TestListUsers_DefaultsAndNullToken(t *testing.T) {
	t.Parallel()

	store := &fakeAdminStore{page: profiles.Page{Profiles: nil, ContinuationToken: ""}}
	h := newTestHandler(store, &fakeTierSync{}, &fakeOfferStore{}, &fakeGenerator{}, time.Now())

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
}

func TestListUsers_InvalidPageSize(t *testing.T) {
	t.Parallel()

	h := newTestHandler(&fakeAdminStore{}, &fakeTierSync{}, &fakeOfferStore{}, &fakeGenerator{}, time.Now())
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
	h := newTestHandler(&fakeAdminStore{}, &fakeTierSync{}, offerStore, gen, now)

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
			h := newTestHandler(&fakeAdminStore{}, &fakeTierSync{}, &fakeOfferStore{}, &fakeGenerator{}, time.Now())
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

func TestBackfillWatchZoneLocation_ReturnsReconciledCounts(t *testing.T) {
	t.Parallel()

	backfiller := &fakeBackfiller{result: watchzones.BackfillResult{Total: 5, Backfilled: 3, AlreadyHad: 2}}
	h := newTestHandler(&fakeAdminStore{}, &fakeTierSync{}, &fakeOfferStore{}, &fakeGenerator{}, time.Now())
	h.watchZones = backfiller

	rec := serve(t, h.backfillWatchZoneLocation, http.MethodPost, "/v1/admin/watchzones/backfill-location", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != `{"total":5,"backfilled":3,"alreadyHad":2}` {
		t.Errorf("body = %s", got)
	}
	if backfiller.calls != 1 {
		t.Errorf("backfiller called %d times, want 1", backfiller.calls)
	}
}

func TestBackfillWatchZoneLocation_StoreErrorIs500(t *testing.T) {
	t.Parallel()

	h := newTestHandler(&fakeAdminStore{}, &fakeTierSync{}, &fakeOfferStore{}, &fakeGenerator{}, time.Now())
	h.watchZones = &fakeBackfiller{err: errors.New("scan boom")}

	rec := serve(t, h.backfillWatchZoneLocation, http.MethodPost, "/v1/admin/watchzones/backfill-location", "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

// TestBackfillWatchZoneLocation_RequiresAdminKey locks the X-Admin-Key gate on
// the route, mirroring the gate on the other admin endpoints: a request with no
// key (or an empty configured key) gets a bodyless 401 and never reaches the
// backfiller.
func TestBackfillWatchZoneLocation_RequiresAdminKey(t *testing.T) {
	t.Parallel()

	backfiller := &fakeBackfiller{}
	h := newTestHandler(&fakeAdminStore{}, &fakeTierSync{}, &fakeOfferStore{}, &fakeGenerator{}, time.Now())
	h.watchZones = backfiller
	gated := requireAdminKey("secret", h.backfillWatchZoneLocation)

	rec := serve(t, gated, http.MethodPost, "/v1/admin/watchzones/backfill-location", "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 with no admin key", rec.Code)
	}
	if backfiller.calls != 0 {
		t.Errorf("backfiller must not run for an unauthenticated request, calls = %d", backfiller.calls)
	}
}

func TestStores_SatisfyInterfaces(t *testing.T) {
	t.Parallel()
	var _ profileAdminStore = profiles.NewAdminStore(nil)
	var _ offerCodeStore = offercodes.NewCosmosStore(nil)
	var _ codeGenerator = offercodes.NewRandomGenerator()
	var _ watchZoneBackfiller = watchzones.NewCosmosStore(nil)
}
