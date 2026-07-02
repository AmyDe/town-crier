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
	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

type fakeCodeStore struct {
	codes   map[string]OfferCode
	saved   *OfferCode
	saveErr error
	// casConflictOnce, when true, causes the first RedeemWithCAS call to return
	// ErrCASPreconditionFailed and then mark the code as already redeemed on
	// subsequent Get calls — driving the "lost CAS race" path in the handler.
	casConflictOnce  bool
	casConflictFired bool
	mu               sync.Mutex
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

// RedeemWithCAS is the CAS-aware redemption path. When casConflictOnce is set
// and the conflict has not yet been fired, it returns ErrCASPreconditionFailed
// and marks the code as redeemed so the handler's re-read observes the redeemed
// state and returns 409. Otherwise it mutates the stored code and returns nil.
func (f *fakeCodeStore) RedeemWithCAS(_ context.Context, canonical, userID string, now time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.casConflictOnce && !f.casConflictFired {
		f.casConflictFired = true
		// Mark the stored code as redeemed so a subsequent Get sees it redeemed.
		if c, ok := f.codes[canonical]; ok {
			uid := "auth0|someone-else"
			c.Redeemed = true
			c.RedeemedByUserID = &uid
			f.codes[canonical] = c
		}
		return platform.ErrCASPreconditionFailed
	}
	c, ok := f.codes[canonical]
	if !ok {
		return ErrNotFound
	}
	if c.IsRedeemed() {
		return ErrAlreadyRedeemed
	}
	if err := c.Redeem(userID, now); err != nil {
		return err
	}
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
	mux := http.NewServeMux()
	Routes(mux, codes, profileStore, auth0, func() time.Time { return now }, slog.New(slog.DiscardHandler))
	req := httptest.NewRequestWithContext(auth.WithSubject(context.Background(), "auth0|u1"),
		http.MethodPost, "/v1/offer-codes/redeem", strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func seededCode(t *testing.T) *fakeCodeStore {
	t.Helper()
	c, err := NewOfferCode("ABCDEFGHJKMN", profiles.TierPro, 30, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewOfferCode: %v", err)
	}
	return &fakeCodeStore{codes: map[string]OfferCode{"ABCDEFGHJKMN": c}}
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
	// The code is persisted as redeemed by the caller.
	if codes.saved == nil || !codes.saved.IsRedeemed() || *codes.saved.RedeemedByUserID != "auth0|u1" {
		t.Errorf("code not saved redeemed: %+v", codes.saved)
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

	codes := &fakeCodeStore{codes: map[string]OfferCode{}}
	rec := serveRedeem(t, codes, &fakeProfileStore{profile: freeProfile(t)}, &fakeTierSync{},
		time.Now(), `{"code":"ZZZZZZZZZZZZ"}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Body.String(); got != `{"error":"invalid_code","message":"Offer code 'ZZZZZZZZZZZZ' was not found."}` {
		t.Errorf("body = %s", got)
	}
}

func TestRedeem_AlreadyRedeemed(t *testing.T) {
	t.Parallel()

	c, _ := NewOfferCode("ABCDEFGHJKMN", profiles.TierPro, 30, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	_ = c.Redeem("auth0|someone", time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC))
	codes := &fakeCodeStore{codes: map[string]OfferCode{"ABCDEFGHJKMN": c}}

	rec := serveRedeem(t, codes, &fakeProfileStore{profile: freeProfile(t)}, &fakeTierSync{},
		time.Now(), `{"code":"ABCDEFGHJKMN"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Body.String(); got != `{"error":"code_already_redeemed","message":"Offer code 'ABCDEFGHJKMN' has already been redeemed."}` {
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
	if codes.saved == nil || !codes.saved.IsRedeemed() || *codes.saved.RedeemedByUserID != "auth0|u1" {
		t.Errorf("code not saved redeemed: %+v", codes.saved)
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

// TestRedeem_CASConflictReturns409 exercises the handler's CAS retry loop: the
// fake store returns ErrCASPreconditionFailed on the first RedeemWithCAS call
// (simulating a concurrent writer winning the race) and marks the code as
// already-redeemed on the subsequent Get re-read, so the handler must surface
// 409 code_already_redeemed.
func TestRedeem_CASConflictReturns409(t *testing.T) {
	t.Parallel()

	codes := seededCode(t)
	codes.casConflictOnce = true
	profile := &fakeProfileStore{profile: freeProfile(t)}
	auth0 := &fakeTierSync{}
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

	rec := serveRedeem(t, codes, profile, auth0, now, `{"code":"abcd-efgh-jkmn"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != `{"error":"code_already_redeemed","message":"Offer code 'ABCDEFGHJKMN' has already been redeemed."}` {
		t.Errorf("body = %s", got)
	}
	// Auth0 must not be called when redemption fails.
	if auth0.calls != 0 {
		t.Errorf("auth0 should not be called on CAS conflict, got %d calls", auth0.calls)
	}
}

// TestRedeem_ConcurrentRedemptionsYieldOneSuccess fires two concurrent redeem
// requests against the same code. Because the fake store serialises RedeemWithCAS
// under its mutex, exactly one caller wins; the loser observes the code as
// already-redeemed on its re-read and receives 409.
func TestRedeem_ConcurrentRedemptionsYieldOneSuccess(t *testing.T) {
	t.Parallel()

	// Build a store whose RedeemWithCAS is concurrency-safe: the first caller
	// wins, the second sees the code already redeemed.
	c, err := NewOfferCode("ABCDEFGHJKMN", profiles.TierPro, 30, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewOfferCode: %v", err)
	}
	codes := &fakeCodeStore{codes: map[string]OfferCode{"ABCDEFGHJKMN": c}}

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
		rec := serveRedeem(t, codes, profile1, auth0a, now, `{"code":"abcd-efgh-jkmn"}`)
		results[0] = rec.Code
	}()
	go func() {
		defer wg.Done()
		rec := serveRedeem(t, codes, profile2, auth0b, now, `{"code":"abcd-efgh-jkmn"}`)
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
