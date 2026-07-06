package offercodes

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// fakeCodeStore is a hand-written codeStore double that mirrors the real
// Postgres store's two-write redemption model: a child-table row per
// (code, user) plus the parent code's RedemptionCount. RedeemWithCAS enforces
// the same invariants the real transaction does — one redemption per user per
// code, and a hard cap at MaxRedemptions — entirely in memory, guarded by a
// mutex so the concurrency test below can drive it from multiple goroutines.
type fakeCodeStore struct {
	codes       map[string]OfferCode
	redemptions map[string][]Redemption // canonical code -> its redemptions
	saved       *OfferCode
	saveErr     error
	mu          sync.Mutex
}

func newFakeCodeStore() *fakeCodeStore {
	return &fakeCodeStore{codes: map[string]OfferCode{}, redemptions: map[string][]Redemption{}}
}

func (f *fakeCodeStore) Get(_ context.Context, canonical string) (OfferCode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.codes[canonical]
	if !ok {
		return OfferCode{}, ErrNotFound
	}
	return c, nil
}

func (f *fakeCodeStore) Save(_ context.Context, c OfferCode) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := c
	f.saved = &cp
	return nil
}

// RedeemWithCAS mirrors PostgresStore.RedeemWithCAS: reject a second
// redemption by the same user (ErrAlreadyRedeemedByUser), reject a code with
// no free slots left (ErrAlreadyRedeemed), otherwise record the redemption and
// bump the counter.
func (f *fakeCodeStore) RedeemWithCAS(_ context.Context, canonical, userID string, now time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	c, ok := f.codes[canonical]
	if !ok {
		return ErrNotFound
	}
	for _, r := range f.redemptions[canonical] {
		if r.UserID != nil && *r.UserID == userID {
			return ErrAlreadyRedeemedByUser
		}
	}
	if c.IsFullyRedeemed() {
		return ErrAlreadyRedeemed
	}

	uid := userID
	at := now
	f.redemptions[canonical] = append(f.redemptions[canonical], Redemption{Code: canonical, UserID: &uid, RedeemedAt: &at})
	c.RedemptionCount++
	f.codes[canonical] = c
	cp := c
	f.saved = &cp
	return nil
}

type fakeProfileStore struct {
	profile *profiles.UserProfile
	getErr  error
	saved   *profiles.UserProfile
}

func (f *fakeProfileStore) Get(_ context.Context, _ string) (*profiles.UserProfile, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.profile, nil
}

func (f *fakeProfileStore) Save(_ context.Context, p *profiles.UserProfile) error {
	f.saved = p
	return nil
}

type fakeTierSync struct {
	gotUserID string
	gotTier   string
	calls     int
}

func (f *fakeTierSync) UpdateSubscriptionTier(_ context.Context, userID, tier string) error {
	f.calls++
	f.gotUserID = userID
	f.gotTier = tier
	return nil
}

func freeProfile(t *testing.T) *profiles.UserProfile {
	t.Helper()
	p, err := profiles.NewProfile("auth0|u1", "u@example.com", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}
	return p
}

func serveRedeem(t *testing.T, codes codeStore, profileStore profileStore, auth0 tierSync, now time.Time, body string) *httptest.ResponseRecorder {
	t.Helper()
	return serveRedeemAs(t, codes, profileStore, auth0, now, body, "auth0|u1")
}

func serveRedeemAs(t *testing.T, codes codeStore, profileStore profileStore, auth0 tierSync, now time.Time, body, userID string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	Routes(mux, codes, profileStore, auth0, func() time.Time { return now }, slog.New(slog.DiscardHandler))
	req := httptest.NewRequestWithContext(auth.WithSubject(context.Background(), userID),
		http.MethodPost, "/v1/offer-codes/redeem", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// seededCode returns a fakeCodeStore holding one single-use (maxRedemptions=1)
// code, the common case exercised by most tests here.
func seededCode(t *testing.T) *fakeCodeStore {
	t.Helper()
	return seededCodeWithCap(t, 1)
}

// seededCodeWithCap returns a fakeCodeStore holding one code minted with the
// given redemption cap.
func seededCodeWithCap(t *testing.T, maxRedemptions int) *fakeCodeStore {
	t.Helper()
	c, err := NewOfferCode("ABCDEFGHJKMN", profiles.TierPro, 30, "campaign", maxRedemptions,
		time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewOfferCode: %v", err)
	}
	store := newFakeCodeStore()
	store.codes["ABCDEFGHJKMN"] = c
	return store
}

func TestRedeem_HappyPath(t *testing.T) {
	t.Parallel()

	codes := seededCode(t)
	profile := &fakeProfileStore{profile: freeProfile(t)}
	auth0 := &fakeTierSync{}
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

	// Mixed-case, hyphenated input normalises to the canonical code.
	rec := serveRedeem(t, codes, profile, auth0, now, `{"code":"abcd-efgh-jkmn"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != `{"tier":"Pro","expiresAt":"2026-07-10T12:00:00+00:00"}` {
		t.Errorf("body = %s", got)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("content-type = %q", ct)
	}
	// The code is persisted as fully redeemed by the caller.
	if codes.saved == nil || !codes.saved.IsFullyRedeemed() {
		t.Errorf("code not saved fully redeemed: %+v", codes.saved)
	}
	// The profile is activated to the granted tier and persisted.
	if profile.saved == nil || profile.saved.Tier != profiles.TierPro || profile.saved.SubscriptionExpiry == nil {
		t.Errorf("profile not activated: %+v", profile.saved)
	}
	// Auth0 is told the new tier exactly once.
	if auth0.calls != 1 || auth0.gotUserID != "auth0|u1" || auth0.gotTier != "Pro" {
		t.Errorf("auth0 sync wrong: %+v", auth0)
	}
}

func TestRedeem_InvalidFormat(t *testing.T) {
	t.Parallel()

	rec := serveRedeem(t, seededCode(t), &fakeProfileStore{profile: freeProfile(t)}, &fakeTierSync{},
		time.Now(), `{"code":"ABCD"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Body.String(); got != `{"error":"invalid_code_format","message":"Offer code must be 12 characters (got 4)."}` {
		t.Errorf("body = %s", got)
	}
}

func TestRedeem_NotFound(t *testing.T) {
	t.Parallel()

	codes := newFakeCodeStore()
	rec := serveRedeem(t, codes, &fakeProfileStore{profile: freeProfile(t)}, &fakeTierSync{},
		time.Now(), `{"code":"ZZZZZZZZZZZZ"}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Body.String(); got != `{"error":"invalid_code","message":"Offer code 'ZZZZZZZZZZZZ' was not found."}` {
		t.Errorf("body = %s", got)
	}
}

// TestRedeem_AlreadyRedeemed covers the fully-consumed fast path: a single-use
// code that some other user already claimed.
func TestRedeem_AlreadyRedeemed(t *testing.T) {
	t.Parallel()

	codes := seededCode(t)
	someoneElse := "auth0|someone"
	at := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	codes.redemptions["ABCDEFGHJKMN"] = []Redemption{{Code: "ABCDEFGHJKMN", UserID: &someoneElse, RedeemedAt: &at}}
	c := codes.codes["ABCDEFGHJKMN"]
	c.RedemptionCount = 1
	codes.codes["ABCDEFGHJKMN"] = c

	rec := serveRedeem(t, codes, &fakeProfileStore{profile: freeProfile(t)}, &fakeTierSync{},
		time.Now(), `{"code":"ABCDEFGHJKMN"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Body.String(); got != `{"error":"code_already_redeemed","message":"Offer code 'ABCDEFGHJKMN' has already been redeemed."}` {
		t.Errorf("body = %s", got)
	}
}

// TestRedeem_AlreadyRedeemedByThisUser is the distinct same-user-twice case:
// the caller already personally redeemed a multi-use code that still has free
// slots for other users, so the fast-path IsFullyRedeemed check does not
// catch it — RedeemWithCAS's ErrAlreadyRedeemedByUser must.
func TestRedeem_AlreadyRedeemedByThisUser(t *testing.T) {
	t.Parallel()

	codes := seededCodeWithCap(t, 3)
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	profile := &fakeProfileStore{profile: freeProfile(t)}
	auth0 := &fakeTierSync{}

	first := serveRedeem(t, codes, profile, auth0, now, `{"code":"ABCDEFGHJKMN"}`)
	if first.Code != http.StatusOK {
		t.Fatalf("first redeem status = %d body = %s", first.Code, first.Body.String())
	}

	// Same user (auth0|u1), same code, second attempt.
	second := serveRedeem(t, codes, &fakeProfileStore{profile: freeProfile(t)}, auth0, now, `{"code":"ABCDEFGHJKMN"}`)
	if second.Code != http.StatusConflict {
		t.Fatalf("second redeem status = %d body = %s", second.Code, second.Body.String())
	}
	if got := second.Body.String(); got != `{"error":"code_already_redeemed","message":"Offer code 'ABCDEFGHJKMN' has already been redeemed."}` {
		t.Errorf("body = %s", got)
	}
}

// TestRedeem_MultiUseAcceptsDistinctUsersUpToCap confirms a maxRedemptions=3
// code accepts three distinct redeemers and then rejects a fourth.
func TestRedeem_MultiUseAcceptsDistinctUsersUpToCap(t *testing.T) {
	t.Parallel()

	codes := seededCodeWithCap(t, 3)
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	auth0 := &fakeTierSync{}

	for i, userID := range []string{"auth0|u1", "auth0|u2", "auth0|u3"} {
		rec := serveRedeemAs(t, codes, &fakeProfileStore{profile: freeProfile(t)}, auth0, now, `{"code":"ABCDEFGHJKMN"}`, userID)
		if rec.Code != http.StatusOK {
			t.Fatalf("redemption %d (%s) status = %d body = %s", i+1, userID, rec.Code, rec.Body.String())
		}
	}

	fourth := serveRedeemAs(t, codes, &fakeProfileStore{profile: freeProfile(t)}, auth0, now, `{"code":"ABCDEFGHJKMN"}`, "auth0|u4")
	if fourth.Code != http.StatusConflict {
		t.Fatalf("fourth redemption status = %d body = %s", fourth.Code, fourth.Body.String())
	}
	if got := fourth.Body.String(); got != `{"error":"code_already_redeemed","message":"Offer code 'ABCDEFGHJKMN' has already been redeemed."}` {
		t.Errorf("body = %s", got)
	}
}

// TestRedeem_AlreadySubscribed covers a currently-active paid subscription
// (SubscriptionExpiry strictly after now): EffectiveTier reports the raw paid
// tier unchanged, so the guard still blocks redemption.
func TestRedeem_AlreadySubscribed(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	paid := freeProfile(t)
	paid.ActivateSubscription(profiles.TierPersonal, now.AddDate(0, 6, 0)) // expiry well in the future

	rec := serveRedeem(t, seededCode(t), &fakeProfileStore{profile: paid}, &fakeTierSync{},
		now, `{"code":"ABCDEFGHJKMN"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Body.String(); got != `{"error":"already_subscribed","message":"User already has an active subscription; offer codes are only available to free-tier users."}` {
		t.Errorf("body = %s", got)
	}
}

// TestRedeem_LapsedSubscriptionCanRedeem is the regression test for tc-jh0o:
// the profile's raw Tier is still paid but SubscriptionExpiry is in the past
// with no grace period, so EffectiveTier collapses to Free and the guard must
// allow the redemption to proceed (and succeed).
func TestRedeem_LapsedSubscriptionCanRedeem(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	lapsed := freeProfile(t)
	lapsed.ActivateSubscription(profiles.TierPro, now.AddDate(0, 0, -1)) // expired yesterday, no grace period

	codes := seededCode(t)
	profile := &fakeProfileStore{profile: lapsed}
	auth0 := &fakeTierSync{}

	rec := serveRedeem(t, codes, profile, auth0, now, `{"code":"ABCDEFGHJKMN"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if codes.saved == nil || !codes.saved.IsFullyRedeemed() {
		t.Errorf("code not saved fully redeemed: %+v", codes.saved)
	}
	if profile.saved == nil || profile.saved.Tier != profiles.TierPro || profile.saved.SubscriptionExpiry == nil {
		t.Errorf("profile not activated to the newly granted tier: %+v", profile.saved)
	}
}

// TestRedeem_ActiveGracePeriodBlocksRedeem covers the App Store billing-retry
// window: SubscriptionExpiry has passed but GracePeriodExpiry is still in the
// future, so EffectiveTier keeps the paid tier and the guard must still block.
func TestRedeem_ActiveGracePeriodBlocksRedeem(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	inGrace := freeProfile(t)
	inGrace.ActivateSubscription(profiles.TierPro, now.AddDate(0, 0, -1)) // expired yesterday
	inGrace.EnterGracePeriod(now.AddDate(0, 0, 1))                        // grace period active until tomorrow

	rec := serveRedeem(t, seededCode(t), &fakeProfileStore{profile: inGrace}, &fakeTierSync{},
		now, `{"code":"ABCDEFGHJKMN"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != `{"error":"already_subscribed","message":"User already has an active subscription; offer codes are only available to free-tier users."}` {
		t.Errorf("body = %s", got)
	}
}

// TestRedeem_ConcurrentRedemptionsYieldOneSuccess fires two concurrent redeem
// requests from two different users against the same single-use code. Because
// the fake store serialises RedeemWithCAS under its mutex, exactly one caller
// wins; the loser gets ErrAlreadyRedeemed and 409.
func TestRedeem_ConcurrentRedemptionsYieldOneSuccess(t *testing.T) {
	t.Parallel()

	codes := seededCode(t)

	profile1 := &fakeProfileStore{profile: freeProfile(t)}
	profile2 := &fakeProfileStore{profile: freeProfile(t)}
	auth0a := &fakeTierSync{}
	auth0b := &fakeTierSync{}
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

	var wg sync.WaitGroup
	results := make([]int, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		rec := serveRedeemAs(t, codes, profile1, auth0a, now, `{"code":"abcd-efgh-jkmn"}`, "auth0|u1")
		results[0] = rec.Code
	}()
	go func() {
		defer wg.Done()
		rec := serveRedeemAs(t, codes, profile2, auth0b, now, `{"code":"abcd-efgh-jkmn"}`, "auth0|u2")
		results[1] = rec.Code
	}()
	wg.Wait()

	successes := 0
	conflicts := 0
	for _, code := range results {
		switch code {
		case http.StatusOK:
			successes++
		case http.StatusConflict:
			conflicts++
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Errorf("want 1 success + 1 conflict, got statuses %v", results)
	}
}
