package offercodes

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

type fakeCodeStore struct {
	codes   map[string]OfferCode
	saved   *OfferCode
	saveErr error
}

func (f *fakeCodeStore) Get(_ context.Context, canonical string) (OfferCode, error) {
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

func TestRedeem_AlreadySubscribed(t *testing.T) {
	t.Parallel()

	paid := freeProfile(t)
	paid.ActivateSubscription(profiles.TierPersonal, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))

	rec := serveRedeem(t, seededCode(t), &fakeProfileStore{profile: paid}, &fakeTierSync{},
		time.Now(), `{"code":"ABCDEFGHJKMN"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Body.String(); got != `{"error":"already_subscribed","message":"User already has an active subscription; offer codes are only available to free-tier users."}` {
		t.Errorf("body = %s", got)
	}
}

func TestCosmosStore_SatisfiesCodeStore(t *testing.T) {
	t.Parallel()
	var _ codeStore = NewCosmosStore(newFakeItems())
}

func TestCosmosStore_SatisfiesProfileStore(t *testing.T) {
	t.Parallel()
	// The real profiles.CosmosStore must satisfy the redeem handler's profileStore.
	var _ profileStore = profiles.NewCosmosStore(nil)
}
