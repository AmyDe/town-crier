package subscriptions

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

const (
	testBundleID = "uk.towncrierapp.mobile"
	testUserID   = "auth0|u1"
)

var testNow = time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)

func futureExpiryMs() int64 { return testNow.AddDate(0, 1, 0).UnixMilli() }
func pastExpiryMs() int64   { return testNow.AddDate(0, -1, 0).UnixMilli() }

// --- fakes ---

type fakeVerifier struct {
	results   map[string]string
	errs      map[string]error
	callCount int
}

func (f *fakeVerifier) VerifyAndDecode(signed string) (string, error) {
	f.callCount++
	if e, ok := f.errs[signed]; ok {
		return "", e
	}
	if j, ok := f.results[signed]; ok {
		return j, nil
	}
	return "", &JWSVerificationError{Message: "unmapped test jws"}
}

type fakeProfileByUser struct {
	profile *profiles.UserProfile
	getErr  error
	saved   *profiles.UserProfile
}

func (f *fakeProfileByUser) Get(context.Context, string) (*profiles.UserProfile, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.profile, nil
}

func (f *fakeProfileByUser) Save(_ context.Context, p *profiles.UserProfile) error {
	f.saved = p
	return nil
}

type fakeProfileByTxn struct {
	// profile is returned for all txn lookups when profilesByTxnID is nil.
	profile *profiles.UserProfile
	// profilesByTxnID maps originalTransactionID to a profile, supporting F2
	// conflict tests where different txn ids resolve to different profiles.
	profilesByTxnID map[string]*profiles.UserProfile
	saved           *profiles.UserProfile
}

func (f *fakeProfileByTxn) GetByOriginalTransactionID(_ context.Context, txnID string) (*profiles.UserProfile, error) {
	if f.profilesByTxnID != nil {
		if p, ok := f.profilesByTxnID[txnID]; ok {
			return p, nil
		}
		return nil, profiles.ErrNotFound
	}
	if f.profile == nil {
		return nil, profiles.ErrNotFound
	}
	return f.profile, nil
}

func (f *fakeProfileByTxn) Save(_ context.Context, p *profiles.UserProfile) error {
	f.saved = p
	return nil
}

type fakeAuth0 struct{ tiers []string }

func (f *fakeAuth0) UpdateSubscriptionTier(_ context.Context, _, tier string) error {
	f.tiers = append(f.tiers, tier)
	return nil
}

type fakeIdempotency struct {
	processed map[string]bool
	marked    []string
}

func newFakeIdempotency() *fakeIdempotency {
	return &fakeIdempotency{processed: map[string]bool{}}
}

func (f *fakeIdempotency) IsProcessed(_ context.Context, uuid string) (bool, error) {
	return f.processed[uuid], nil
}

func (f *fakeIdempotency) MarkProcessed(_ context.Context, uuid string) error {
	f.marked = append(f.marked, uuid)
	return nil
}

// --- harness ---

type testDeps struct {
	verifier    *fakeVerifier
	byUser      *fakeProfileByUser
	byTxn       *fakeProfileByTxn
	auth0       *fakeAuth0
	idempotency *fakeIdempotency
	mux         *http.ServeMux
}

// testAllowedEnvs is the default allowlist used by newTestDeps. It mirrors a
// production config that only accepts Production-environment transactions.
var testAllowedEnvs = []string{"Production"}

func newTestDeps() *testDeps {
	return newTestDepsWithEnvs(testAllowedEnvs)
}

func newTestDepsWithEnvs(allowedEnvs []string) *testDeps {
	d := &testDeps{
		verifier:    &fakeVerifier{results: map[string]string{}, errs: map[string]error{}},
		byUser:      &fakeProfileByUser{},
		byTxn:       &fakeProfileByTxn{},
		auth0:       &fakeAuth0{},
		idempotency: newFakeIdempotency(),
		mux:         http.NewServeMux(),
	}
	Routes(d.mux, d.verifier, d.byUser, d.byTxn, d.auth0, d.idempotency, testBundleID, allowedEnvs,
		func() time.Time { return testNow }, slog.New(slog.DiscardHandler))
	return d
}

func (d *testDeps) serve(t *testing.T, path, body string, authed bool) *httptest.ResponseRecorder {
	t.Helper()
	ctx := context.Background()
	if authed {
		ctx = auth.WithSubject(ctx, testUserID)
	}
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	d.mux.ServeHTTP(rec, req)
	return rec
}

func freshProfile(t *testing.T) *profiles.UserProfile {
	t.Helper()
	p, err := profiles.NewProfile(testUserID, "u@example.com", testNow)
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}
	return p
}

func txnJSON(productID, bundleID, origTxn string, expiresMs int64) string {
	return txnJSONEnv(productID, bundleID, origTxn, expiresMs, "Production")
}

func txnJSONEnv(productID, bundleID, origTxn string, expiresMs int64, environment string) string {
	return fmt.Sprintf(`{"transactionId":"t1","originalTransactionId":%q,"productId":%q,"bundleId":%q,"purchaseDate":1,"expiresDate":%d,"environment":%q}`,
		origTxn, productID, bundleID, expiresMs, environment)
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) apiErrorResponse {
	t.Helper()
	var e apiErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("decode error body %q: %v", rec.Body.String(), err)
	}
	return e
}

// --- verify tests ---

func TestVerify_PurchaseActivatesPro(t *testing.T) {
	t.Parallel()
	d := newTestDeps()
	d.byUser.profile = freshProfile(t)
	d.verifier.results["JWS_PRO"] = txnJSON(ProductProMonthly, testBundleID, "orig-1", futureExpiryMs())

	rec := d.serve(t, "/v1/subscriptions/verify", `{"signedTransaction":"JWS_PRO"}`, true)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp verifyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Tier != "Pro" {
		t.Errorf("tier = %q, want Pro", resp.Tier)
	}
	if resp.WatchZoneLimit != 2147483647 {
		t.Errorf("watchZoneLimit = %d, want 2147483647", resp.WatchZoneLimit)
	}
	if len(resp.Entitlements) != 3 {
		t.Errorf("entitlements = %v", resp.Entitlements)
	}
	if d.byUser.saved == nil || d.byUser.saved.Tier != profiles.TierPro {
		t.Error("profile not saved as Pro")
	}
	if d.byUser.saved.OriginalTransactionID == nil || *d.byUser.saved.OriginalTransactionID != "orig-1" {
		t.Error("original transaction id not linked")
	}
	if len(d.auth0.tiers) != 1 || d.auth0.tiers[0] != "Pro" {
		t.Errorf("auth0 sync = %v, want [Pro]", d.auth0.tiers)
	}
}

func TestVerify_RestoreAllLapsedExpires(t *testing.T) {
	t.Parallel()
	d := newTestDeps()
	p := freshProfile(t)
	p.ActivateSubscription(profiles.TierPro, testNow.AddDate(1, 0, 0))
	d.byUser.profile = p
	// Both restored transactions are already expired — the user is Free.
	d.verifier.results["A"] = txnJSON(ProductProMonthly, testBundleID, "o1", pastExpiryMs())
	d.verifier.results["B"] = txnJSON(ProductPersonalMonthly, testBundleID, "o2", pastExpiryMs())

	rec := d.serve(t, "/v1/subscriptions/verify", `{"signedTransactions":["A","B"]}`, true)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp verifyResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Tier != "Free" || resp.WatchZoneLimit != 1 {
		t.Errorf("tier=%q limit=%d, want Free/1", resp.Tier, resp.WatchZoneLimit)
	}
	if resp.SubscriptionExpiry != nil {
		t.Errorf("subscriptionExpiry = %v, want null", resp.SubscriptionExpiry)
	}
}

func TestVerify_ErrorContract(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		body       string
		setup      func(d *testDeps)
		wantStatus int
		wantError  string
	}{
		{
			name:       "malformed body",
			body:       "{not json",
			setup:      func(d *testDeps) { d.byUser.profile = nil },
			wantStatus: http.StatusBadRequest,
			wantError:  "malformed_request",
		},
		{
			name:       "no signed transactions",
			body:       "{}",
			wantStatus: http.StatusBadRequest,
			wantError:  "malformed_request",
		},
		{
			name: "jws failure",
			body: `{"signedTransaction":"BAD"}`,
			setup: func(d *testDeps) {
				d.byUser.profile = freshProfile(t)
				d.verifier.errs["BAD"] = &JWSVerificationError{Message: "tampered"}
			},
			wantStatus: http.StatusUnauthorized,
			wantError:  "invalid_transaction",
		},
		{
			name: "bundle mismatch",
			body: `{"signedTransaction":"WRONGBUNDLE"}`,
			setup: func(d *testDeps) {
				d.byUser.profile = freshProfile(t)
				d.verifier.results["WRONGBUNDLE"] = txnJSON(ProductProMonthly, "com.someone.else", "o", futureExpiryMs())
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_transaction_payload",
		},
		{
			name: "unknown product",
			body: `{"signedTransaction":"UNKNOWN"}`,
			setup: func(d *testDeps) {
				d.byUser.profile = freshProfile(t)
				d.verifier.results["UNKNOWN"] = txnJSON("com.example.bogus", testBundleID, "o", futureExpiryMs())
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_transaction_payload",
		},
		{
			name: "user not found",
			body: `{"signedTransaction":"JWS"}`,
			setup: func(d *testDeps) {
				d.byUser.getErr = profiles.ErrNotFound
				d.verifier.results["JWS"] = txnJSON(ProductProMonthly, testBundleID, "o", futureExpiryMs())
			},
			wantStatus: http.StatusNotFound,
			wantError:  "user_not_found",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := newTestDeps()
			if tc.setup != nil {
				tc.setup(d)
			}
			rec := d.serve(t, "/v1/subscriptions/verify", tc.body, true)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body=%s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if got := decodeError(t, rec).Error; got != tc.wantError {
				t.Errorf("error = %q, want %q", got, tc.wantError)
			}
		})
	}
}

// --- webhook tests ---

func notificationJSON(notifType, uuid, signedTxn string) string {
	return fmt.Sprintf(`{"notificationType":%q,"notificationUUID":%q,"data":{"signedTransactionInfo":%q}}`,
		notifType, uuid, signedTxn)
}

func TestWebhook_SubscribedActivatesProfile(t *testing.T) {
	t.Parallel()
	d := newTestDeps()
	d.byTxn.profile = freshProfile(t)
	d.verifier.results["hdr.OUTER.sig"] = notificationJSON("SUBSCRIBED", "uuid-1", "INNER")
	d.verifier.results["INNER"] = txnJSON(ProductProMonthly, testBundleID, "orig-1", futureExpiryMs())

	rec := d.serve(t, "/v1/webhooks/appstore", `{"signedPayload":"hdr.OUTER.sig"}`, false)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if d.byTxn.saved == nil || d.byTxn.saved.Tier != profiles.TierPro {
		t.Error("profile not activated to Pro")
	}
	if len(d.auth0.tiers) != 1 || d.auth0.tiers[0] != "Pro" {
		t.Errorf("auth0 sync = %v", d.auth0.tiers)
	}
	if len(d.idempotency.marked) != 1 || d.idempotency.marked[0] != "uuid-1" {
		t.Errorf("marked = %v, want [uuid-1]", d.idempotency.marked)
	}
}

func TestWebhook_DuplicateIsNoOp(t *testing.T) {
	t.Parallel()
	d := newTestDeps()
	d.byTxn.profile = freshProfile(t)
	d.idempotency.processed["uuid-1"] = true
	d.verifier.results["hdr.OUTER.sig"] = notificationJSON("SUBSCRIBED", "uuid-1", "INNER")
	d.verifier.results["INNER"] = txnJSON(ProductProMonthly, testBundleID, "orig-1", futureExpiryMs())

	rec := d.serve(t, "/v1/webhooks/appstore", `{"signedPayload":"hdr.OUTER.sig"}`, false)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if d.byTxn.saved != nil {
		t.Error("duplicate should not save the profile")
	}
	if len(d.idempotency.marked) != 0 {
		t.Errorf("duplicate should not re-mark, got %v", d.idempotency.marked)
	}
}

func TestWebhook_UnknownSubscriberStillMarksProcessed(t *testing.T) {
	t.Parallel()
	d := newTestDeps()
	d.byTxn.profile = nil // no matching subscriber
	d.verifier.results["hdr.OUTER.sig"] = notificationJSON("DID_RENEW", "uuid-2", "INNER")
	d.verifier.results["INNER"] = txnJSON(ProductProMonthly, testBundleID, "orig-x", futureExpiryMs())

	rec := d.serve(t, "/v1/webhooks/appstore", `{"signedPayload":"hdr.OUTER.sig"}`, false)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if d.byTxn.saved != nil {
		t.Error("no subscriber should mean no save")
	}
	if len(d.idempotency.marked) != 1 {
		t.Errorf("should still mark processed, got %v", d.idempotency.marked)
	}
}

func TestWebhook_ErrorContract(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		body       string
		setup      func(d *testDeps)
		wantStatus int
		wantError  string
	}{
		{"malformed body", "{nope", nil, http.StatusBadRequest, "malformed_request"},
		{"empty payload", `{"signedPayload":""}`, nil, http.StatusBadRequest, "malformed_request"},
		{
			name: "jws failure",
			body: `{"signedPayload":"hdr.BAD.sig"}`,
			setup: func(d *testDeps) {
				d.verifier.errs["hdr.BAD.sig"] = &JWSVerificationError{Message: "bad chain"}
			},
			wantStatus: http.StatusUnauthorized,
			wantError:  "invalid_notification",
		},
		{
			name: "malformed notification payload",
			body: `{"signedPayload":"hdr.OUTER.sig"}`,
			setup: func(d *testDeps) {
				d.verifier.results["hdr.OUTER.sig"] = `{"notificationType":"SUBSCRIBED","notificationUUID":"u","data":{}}`
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_notification_payload",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := newTestDeps()
			if tc.setup != nil {
				tc.setup(d)
			}
			rec := d.serve(t, "/v1/webhooks/appstore", tc.body, false)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body=%s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if got := decodeError(t, rec).Error; got != tc.wantError {
				t.Errorf("error = %q, want %q", got, tc.wantError)
			}
		})
	}
}

// --- applyNotification (pure) tests ---

func TestApplyNotification(t *testing.T) {
	t.Parallel()
	future := testNow.AddDate(0, 1, 0).UTC()
	tests := []struct {
		name        string
		notifType   string
		subtype     string
		productID   string
		startTier   profiles.SubscriptionTier
		wantErr     bool
		wantChanged bool
		wantTier    profiles.SubscriptionTier
		wantGrace   bool
		wantExpiry  bool // when true, assert SubscriptionExpiry == future
	}{
		{"subscribed", "SUBSCRIBED", "", ProductProMonthly, profiles.TierFree, false, true, profiles.TierPro, false, false},
		{"offer redeemed", "OFFER_REDEEMED", "", ProductProMonthly, profiles.TierFree, false, true, profiles.TierPro, false, false},
		{"did renew same product keeps tier", "DID_RENEW", "", ProductProMonthly, profiles.TierPro, false, true, profiles.TierPro, false, true},
		{"did renew lower product downgrades tier", "DID_RENEW", "", ProductPersonalMonthly, profiles.TierPro, false, true, profiles.TierPersonal, false, true},
		{"did renew unknown product errors", "DID_RENEW", "", "uk.towncrierapp.mystery.monthly", profiles.TierPro, true, false, profiles.TierPro, false, false},
		{"upgrade", "DID_CHANGE_RENEWAL_PREF", "UPGRADE", ProductProMonthly, profiles.TierPersonal, false, true, profiles.TierPro, false, false},
		{"downgrade no change", "DID_CHANGE_RENEWAL_PREF", "DOWNGRADE", ProductProMonthly, profiles.TierPro, false, false, profiles.TierPro, false, false},
		{"grace period", "DID_FAIL_TO_RENEW", "GRACE_PERIOD", ProductProMonthly, profiles.TierPro, false, true, profiles.TierPro, true, false},
		{"fail to renew expires", "DID_FAIL_TO_RENEW", "", ProductProMonthly, profiles.TierPro, false, true, profiles.TierFree, false, false},
		{"expired", "EXPIRED", "", ProductProMonthly, profiles.TierPro, false, true, profiles.TierFree, false, false},
		{"revoke", "REVOKE", "", ProductProMonthly, profiles.TierPro, false, true, profiles.TierFree, false, false},
		{"unknown type ignored", "PRICE_INCREASE", "", ProductProMonthly, profiles.TierPro, false, false, profiles.TierPro, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := freshProfile(t)
			p.Tier = tc.startTier
			notif := DecodedNotification{NotificationType: tc.notifType, Subtype: tc.subtype}
			txn := DecodedTransaction{ProductID: tc.productID, ExpiresDate: future}

			changed, err := applyNotification(p, notif, txn)
			if tc.wantErr {
				if err == nil {
					t.Fatal("applyNotification: want error, got nil")
				}
				if changed {
					t.Error("changed = true, want false on error")
				}
				if p.Tier != tc.startTier {
					t.Errorf("tier = %v after error, want unchanged %v", p.Tier, tc.startTier)
				}
				if p.SubscriptionExpiry != nil {
					t.Errorf("expiry = %v after error, want unmutated (nil)", p.SubscriptionExpiry)
				}
				if p.GracePeriodExpiry != nil {
					t.Error("grace period set after error, want unmutated")
				}
				return
			}
			if err != nil {
				t.Fatalf("applyNotification: %v", err)
			}
			if changed != tc.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tc.wantChanged)
			}
			if p.Tier != tc.wantTier {
				t.Errorf("tier = %v, want %v", p.Tier, tc.wantTier)
			}
			if (p.GracePeriodExpiry != nil) != tc.wantGrace {
				t.Errorf("grace set = %v, want %v", p.GracePeriodExpiry != nil, tc.wantGrace)
			}
			if tc.wantExpiry && (p.SubscriptionExpiry == nil || !p.SubscriptionExpiry.Equal(future)) {
				t.Errorf("expiry = %v, want %v", p.SubscriptionExpiry, future)
			}
		})
	}
}

// --- F1: environment allowlist tests ---

func TestVerify_RejectsSandboxTransactionInProduction(t *testing.T) {
	t.Parallel()
	d := newTestDeps() // allowedEnvs = ["Production"]
	d.byUser.profile = freshProfile(t)
	d.verifier.results["JWS_SANDBOX"] = txnJSONEnv(ProductProMonthly, testBundleID, "orig-1", futureExpiryMs(), "Sandbox")

	rec := d.serve(t, "/v1/subscriptions/verify", `{"signedTransaction":"JWS_SANDBOX"}`, true)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	if got := decodeError(t, rec).Error; got != "invalid_transaction_payload" {
		t.Errorf("error = %q, want invalid_transaction_payload", got)
	}
	if d.byUser.saved != nil {
		t.Error("profile must not be saved on environment mismatch")
	}
	if len(d.auth0.tiers) != 0 {
		t.Error("auth0 must not be synced on environment mismatch")
	}
}

func TestVerify_AcceptsProductionTransaction(t *testing.T) {
	t.Parallel()
	d := newTestDeps() // allowedEnvs = ["Production"]
	d.byUser.profile = freshProfile(t)
	d.verifier.results["JWS_PROD"] = txnJSON(ProductProMonthly, testBundleID, "orig-1", futureExpiryMs())

	rec := d.serve(t, "/v1/subscriptions/verify", `{"signedTransaction":"JWS_PROD"}`, true)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if d.byUser.saved == nil || d.byUser.saved.Tier != profiles.TierPro {
		t.Error("profile not saved as Pro")
	}
}

func TestVerify_AcceptsSandboxWhenConfiguredForSandbox(t *testing.T) {
	t.Parallel()
	d := newTestDepsWithEnvs([]string{"Sandbox"})
	d.byUser.profile = freshProfile(t)
	d.verifier.results["JWS_SANDBOX"] = txnJSONEnv(ProductProMonthly, testBundleID, "orig-1", futureExpiryMs(), "Sandbox")

	rec := d.serve(t, "/v1/subscriptions/verify", `{"signedTransaction":"JWS_SANDBOX"}`, true)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if d.byUser.saved == nil || d.byUser.saved.Tier != profiles.TierPro {
		t.Error("profile not saved as Pro")
	}
}

func TestVerify_AcceptsBothEnvironmentsWhenAllowlistHasTwo(t *testing.T) {
	t.Parallel()
	allowBoth := []string{"Sandbox", "Production"}

	t.Run("accepts Production", func(t *testing.T) {
		t.Parallel()
		d := newTestDepsWithEnvs(allowBoth)
		d.byUser.profile = freshProfile(t)
		d.verifier.results["JWS_PROD"] = txnJSON(ProductProMonthly, testBundleID, "orig-1", futureExpiryMs())

		rec := d.serve(t, "/v1/subscriptions/verify", `{"signedTransaction":"JWS_PROD"}`, true)
		if rec.Code != http.StatusOK {
			t.Fatalf("Production status = %d, body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("accepts Sandbox", func(t *testing.T) {
		t.Parallel()
		d := newTestDepsWithEnvs(allowBoth)
		d.byUser.profile = freshProfile(t)
		d.verifier.results["JWS_SBX"] = txnJSONEnv(ProductProMonthly, testBundleID, "orig-2", futureExpiryMs(), "Sandbox")

		rec := d.serve(t, "/v1/subscriptions/verify", `{"signedTransaction":"JWS_SBX"}`, true)
		if rec.Code != http.StatusOK {
			t.Fatalf("Sandbox status = %d, body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestWebhook_IgnoresSandboxTransactionInProduction(t *testing.T) {
	t.Parallel()
	d := newTestDeps() // allowedEnvs = ["Production"]
	d.byTxn.profile = freshProfile(t)
	d.verifier.results["hdr.OUTER.sig"] = notificationJSON("SUBSCRIBED", "uuid-env1", "INNER_SBX")
	d.verifier.results["INNER_SBX"] = txnJSONEnv(ProductProMonthly, testBundleID, "orig-1", futureExpiryMs(), "Sandbox")

	rec := d.serve(t, "/v1/webhooks/appstore", `{"signedPayload":"hdr.OUTER.sig"}`, false)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if d.byTxn.saved != nil {
		t.Error("profile must not be mutated when webhook environment mismatches")
	}
	if len(d.auth0.tiers) != 0 {
		t.Error("auth0 must not be synced when webhook environment mismatches")
	}
	// Notification must still be marked processed (Apple should not retry).
	if len(d.idempotency.marked) != 1 || d.idempotency.marked[0] != "uuid-env1" {
		t.Errorf("should still mark processed, got %v", d.idempotency.marked)
	}
}

// --- F2: transaction ownership tests ---

func TestVerify_RejectsTransactionOwnedByAnotherUser(t *testing.T) {
	t.Parallel()
	d := newTestDeps()
	d.byUser.profile = freshProfile(t)

	// Another user already owns "orig-claimed".
	otherUser, err := profiles.NewProfile("auth0|other", "other@example.com", testNow)
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}
	d.byTxn.profilesByTxnID = map[string]*profiles.UserProfile{
		"orig-claimed": otherUser,
	}
	d.verifier.results["JWS_CONFLICT"] = txnJSON(ProductProMonthly, testBundleID, "orig-claimed", futureExpiryMs())

	rec := d.serve(t, "/v1/subscriptions/verify", `{"signedTransaction":"JWS_CONFLICT"}`, true)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (body=%s)", rec.Code, rec.Body.String())
	}
	if got := decodeError(t, rec).Error; got != "transaction_already_claimed" {
		t.Errorf("error = %q, want transaction_already_claimed", got)
	}
	if d.byUser.saved != nil {
		t.Error("caller profile must not be saved on conflict")
	}
	if len(d.auth0.tiers) != 0 {
		t.Error("auth0 must not be synced on conflict")
	}
}

func TestVerify_AllowsReVerifyBySameOwner(t *testing.T) {
	t.Parallel()
	d := newTestDeps()
	p := freshProfile(t) // UserID = testUserID = "auth0|u1"
	d.byUser.profile = p
	// Same user already owns the tx id — idempotent re-verify should succeed.
	d.byTxn.profilesByTxnID = map[string]*profiles.UserProfile{
		"orig-1": p,
	}
	d.verifier.results["JWS_REVERIFY"] = txnJSON(ProductProMonthly, testBundleID, "orig-1", futureExpiryMs())

	rec := d.serve(t, "/v1/subscriptions/verify", `{"signedTransaction":"JWS_REVERIFY"}`, true)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestVerify_AllowsFirstTimeClaim(t *testing.T) {
	t.Parallel()
	d := newTestDeps()
	d.byUser.profile = freshProfile(t)
	// byTxn returns ErrNotFound for unknown ids (default fakeProfileByTxn behaviour).
	d.verifier.results["JWS_FIRST"] = txnJSON(ProductProMonthly, testBundleID, "orig-new", futureExpiryMs())

	rec := d.serve(t, "/v1/subscriptions/verify", `{"signedTransaction":"JWS_FIRST"}`, true)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if d.byUser.saved == nil || d.byUser.saved.Tier != profiles.TierPro {
		t.Error("profile not saved as Pro on first claim")
	}
}

// --- F7: webhook cheap pre-check tests ---

// TestWebhook_RejectsMalformedPayloadBeforeVerify asserts that a SignedPayload
// that is not a valid compact JWS (header.payload.signature) is rejected with
// 400 malformed_request WITHOUT the JWS verifier being invoked at all. The
// verifier call count proves the expensive crypto path is never entered.
func TestWebhook_RejectsMalformedPayloadBeforeVerify(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		payload string
	}{
		{"no dots", "nodots"},
		{"one dot", "a.b"},
		{"three dots", "a.b.c.d"},
		{"empty first segment", ".b.c"},
		{"empty second segment", "a..c"},
		{"empty third segment", "a.b."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := newTestDeps()
			body := `{"signedPayload":"` + tc.payload + `"}`
			rec := d.serve(t, "/v1/webhooks/appstore", body, false)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
			}
			if got := decodeError(t, rec).Error; got != "malformed_request" {
				t.Errorf("error = %q, want malformed_request", got)
			}
			if d.verifier.callCount != 0 {
				t.Errorf("verifier called %d time(s), want 0 — expensive crypto must not run on malformed input", d.verifier.callCount)
			}
		})
	}
}

// TestWebhook_RejectsOversizedPayloadBeforeVerify asserts that a SignedPayload
// exceeding maxWebhookPayloadBytes is rejected with 400 malformed_request
// WITHOUT invoking the JWS verifier. The verifier call count proves the
// expensive crypto path is never entered.
func TestWebhook_RejectsOversizedPayloadBeforeVerify(t *testing.T) {
	t.Parallel()
	d := newTestDeps()

	// Build a syntactically valid 3-segment JWS that is over the size bound.
	// Each segment is a non-empty base64url-like string; the content does not
	// need to be valid base64 — the size check must fire before decode.
	bigSeg := strings.Repeat("A", maxWebhookPayloadBytes+1)
	oversized := bigSeg + "." + bigSeg + "." + bigSeg

	body := `{"signedPayload":"` + oversized + `"}`
	// Use a large MaxBytesReader so the outer body cap doesn't fire first.
	// The serve helper applies the handler's own body limit; we bypass it here
	// by posting a raw request directly to the mux.
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/webhooks/appstore", strings.NewReader(body))
	rec := httptest.NewRecorder()
	d.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	if got := decodeError(t, rec).Error; got != "malformed_request" {
		t.Errorf("error = %q, want malformed_request", got)
	}
	if d.verifier.callCount != 0 {
		t.Errorf("verifier called %d time(s), want 0 — expensive crypto must not run on oversized input", d.verifier.callCount)
	}
}

// TestWebhook_WellFormedPayloadReachesVerifier is a positive regression test:
// a structurally valid 3-segment JWS within the size bound must still reach
// the verifier (no regression on the happy path introduced by the pre-check).
func TestWebhook_WellFormedPayloadReachesVerifier(t *testing.T) {
	t.Parallel()
	d := newTestDeps()
	d.byTxn.profile = freshProfile(t)
	d.verifier.results["hdr.pay.sig"] = notificationJSON("SUBSCRIBED", "uuid-wf1", "INNER")
	d.verifier.results["INNER"] = txnJSON(ProductProMonthly, testBundleID, "orig-wf1", futureExpiryMs())

	rec := d.serve(t, "/v1/webhooks/appstore", `{"signedPayload":"hdr.pay.sig"}`, false)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if d.verifier.callCount == 0 {
		t.Error("verifier was not called — well-formed payload must reach the verifier")
	}
}
