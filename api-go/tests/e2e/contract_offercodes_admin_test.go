//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

// Contract scenarios for the iteration-8 offer-code and admin surface.
//
// The deterministic, secret-free error paths (redeem invalid-format 400 /
// unknown-code 404, admin no-key 401) are diffed without any shared secret.
//
// The remaining authed + stateful paths (tc-52t6) need the shared ADMIN_API_KEY
// the Go app is now deployed with (infra: EnvironmentStack.cs; CI: pr-gate.yml
// go-contract job). They skip when ADMIN_API_KEY is unset so plain local runs
// stay green:
//   - admin grant/list/generate behind X-Admin-Key, diffed structurally where a
//     value is non-deterministic (random codes, opaque continuation tokens);
//   - the offer-code redeem happy path, which mutates the shared integration
//     user's tier and consumes codes — made idempotent across runs by capturing
//     and restoring the user's original tier.

func TestContract_OfferCodeRedeem_Errors(t *testing.T) {
	dotnetURL := baseURL(t, "DOTNET_BASE_URL")
	goURL := baseURL(t, "GO_BASE_URL")
	token := integrationToken(t)
	client := &http.Client{Timeout: requestTimeout}

	// A malformed code is rejected at the boundary with the invalid_code_format
	// envelope, before any lookup or write.
	t.Run("invalid format", func(t *testing.T) {
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodPost, "/v1/offer-codes/redeem", token, `{"code":"ABCD"}`)
	})

	// A well-formed code that does not exist is a 404 invalid_code. ZZZZZZZZZZZZ
	// is a valid canonical code that is astronomically unlikely to have been
	// minted (60-bit random space), so it is absent on both APIs' shared Cosmos.
	t.Run("unknown code", func(t *testing.T) {
		diffWatchZone(t, client, dotnetURL, goURL, http.MethodPost, "/v1/offer-codes/redeem", token, `{"code":"ZZZZZZZZZZZZ"}`)
	})
}

func TestContract_AdminRequiresKey(t *testing.T) {
	dotnetURL := baseURL(t, "DOTNET_BASE_URL")
	goURL := baseURL(t, "GO_BASE_URL")
	client := &http.Client{Timeout: requestTimeout}

	// The admin endpoints are anonymous to Auth0 but gated by X-Admin-Key. With no
	// key, GET /v1/admin/users returns a bodyless 401 (the PascalCase envelope
	// backfilled) and — unlike the Auth0 fallback-deny — no WWW-Authenticate
	// header. (Only the GET route is exercised here; the PUT/POST admin routes
	// would method-mismatch under diffChallenge's GET.)
	diffChallenge(t, client, dotnetURL, goURL, "/v1/admin/users")
}

// TestContract_AdminAuthedSurface diffs the X-Admin-Key-gated admin endpoints.
// Every scenario here is non-destructive: the grant 404 misses, the validation
// 400s short-circuit before any write, the list reads the shared Cosmos, and the
// generate-success case writes one inert (unredeemed) code per API.
func TestContract_AdminAuthedSurface(t *testing.T) {
	dotnetURL := baseURL(t, "DOTNET_BASE_URL")
	goURL := baseURL(t, "GO_BASE_URL")
	key := adminKey(t)
	client := &http.Client{Timeout: requestTimeout}

	// Grant to a random, certainly-absent email -> bodyless 404 on both
	// (GetByEmail miss). Authenticated by the key but mutates nothing.
	t.Run("PUT grant unknown email -> 404", func(t *testing.T) {
		body := fmt.Sprintf(`{"email":"missing-%s@example.invalid","tier":"Pro"}`, newZoneSuffix(t))
		diffAdmin(t, client, dotnetURL, goURL, http.MethodPut, "/v1/admin/subscriptions", key, body)
	})

	// An unparseable tier is a bodyless 400 before any lookup.
	t.Run("PUT grant invalid tier -> 400", func(t *testing.T) {
		diffAdmin(t, client, dotnetURL, goURL, http.MethodPut, "/v1/admin/subscriptions", key, `{"email":"x@example.com","tier":"Platinum"}`)
	})

	// Generate validation 400s: each guard short-circuits before any Cosmos
	// write, so the ApiErrorResponse bodies (including the .NET runtime's
	// "(Parameter 'command')" / "Actual value was N." suffixes) diff exactly.
	t.Run("POST generate count out of range -> 400", func(t *testing.T) {
		diffAdmin(t, client, dotnetURL, goURL, http.MethodPost, "/v1/admin/offer-codes", key, `{"count":0,"tier":"Pro","durationDays":30}`)
	})
	t.Run("POST generate free tier -> 400", func(t *testing.T) {
		diffAdmin(t, client, dotnetURL, goURL, http.MethodPost, "/v1/admin/offer-codes", key, `{"count":1,"tier":"Free","durationDays":30}`)
	})
	t.Run("POST generate duration out of range -> 400", func(t *testing.T) {
		diffAdmin(t, client, dotnetURL, goURL, http.MethodPost, "/v1/admin/offer-codes", key, `{"count":1,"tier":"Pro","durationDays":0}`)
	})

	// Generate success: a single Personal code. The codes are random so the
	// bodies cannot be byte-diffed; assert both APIs agree on the 200 +
	// text/plain content-type and emit exactly one display-formatted code line.
	t.Run("POST generate success content-type", func(t *testing.T) {
		body := `{"count":1,"tier":"Personal","durationDays":30}`
		want := adminRequest(t, client, dotnetURL, http.MethodPost, "/v1/admin/offer-codes", key, body)
		got := adminRequest(t, client, goURL, http.MethodPost, "/v1/admin/offer-codes", key, body)
		if want.status != http.StatusOK || got.status != http.StatusOK {
			t.Fatalf("status: go=%d dotnet=%d", got.status, want.status)
		}
		if got.contentType != want.contentType {
			t.Errorf("content-type: go=%q dotnet=%q", got.contentType, want.contentType)
		}
		if !strings.HasPrefix(got.contentType, "text/plain") {
			t.Errorf("generate must be text/plain, go=%q", got.contentType)
		}
		assertOneOfferCodeLine(t, "go", got.body)
		assertOneOfferCodeLine(t, "dotnet", want.body)
	})

	// List users filtered to the integration test user: both read the same
	// Cosmos, so the items match as a set (sorted by userId before diffing).
	// Continuation tokens are opaque and SDK-specific, so they are not compared.
	t.Run("GET users (authed, set-equal)", func(t *testing.T) {
		email := integrationEmail(t)
		path := "/v1/admin/users?pageSize=100&search=" + url.QueryEscape(email)
		want := adminRequest(t, client, dotnetURL, http.MethodGet, path, key, "")
		got := adminRequest(t, client, goURL, http.MethodGet, path, key, "")
		if got.status != want.status {
			t.Fatalf("status: go=%d dotnet=%d (go body %s)", got.status, want.status, got.body)
		}
		if got.contentType != want.contentType {
			t.Errorf("content-type: go=%q dotnet=%q", got.contentType, want.contentType)
		}
		if !jsonEqual(t, adminUserItemsSorted(t, got.body), adminUserItemsSorted(t, want.body)) {
			t.Errorf("items: go=%s dotnet=%s", got.body, want.body)
		}
	})
}

// TestContract_OfferCodeRedeem_HappyPath diffs a successful redemption on both
// APIs. Both share one Cosmos, so a single profile and code store back both —
// a code redeemed on .NET cannot be redeemed again on Go. The test therefore
// mints two codes (one per API) and forces the user back to Free between the two
// redemptions. The user's original tier is captured up front and restored on
// cleanup, so the shared user is left exactly as found regardless of outcome.
func TestContract_OfferCodeRedeem_HappyPath(t *testing.T) {
	dotnetURL := baseURL(t, "DOTNET_BASE_URL")
	goURL := baseURL(t, "GO_BASE_URL")
	token := integrationToken(t)
	key := adminKey(t)
	email := integrationEmail(t)
	client := &http.Client{Timeout: requestTimeout}

	originalTier := currentTier(t, client, dotnetURL, key, email)
	t.Cleanup(func() { grantTier(t, client, dotnetURL, key, email, originalTier) })

	// Mint two Personal codes (one per API) in the shared Cosmos via .NET.
	codes := generateCodes(t, client, dotnetURL, key, 2, "Personal", 30)
	if len(codes) != 2 {
		t.Fatalf("expected 2 generated codes, got %d", len(codes))
	}

	grantTier(t, client, dotnetURL, key, email, "Free")
	dotnetResp := watchZoneRequest(t, client, dotnetURL, http.MethodPost, "/v1/offer-codes/redeem", token, redeemBody(codes[0]))
	if dotnetResp.status != http.StatusOK {
		t.Fatalf("dotnet redeem: status %d body %s", dotnetResp.status, dotnetResp.body)
	}

	grantTier(t, client, dotnetURL, key, email, "Free")
	goResp := watchZoneRequest(t, client, goURL, http.MethodPost, "/v1/offer-codes/redeem", token, redeemBody(codes[1]))
	if goResp.status != http.StatusOK {
		t.Fatalf("go redeem: status %d body %s", goResp.status, goResp.body)
	}

	// Structural diff: status (asserted above) + content-type + the tier field.
	// expiresAt is now()+durationDays computed independently on each API, so it
	// differs by the inter-request delay; it is asserted present and parseable,
	// not equal.
	if goResp.contentType != dotnetResp.contentType {
		t.Errorf("content-type: go=%q dotnet=%q", goResp.contentType, dotnetResp.contentType)
	}
	gotTier, gotExpiry := redeemFields(t, goResp.body)
	wantTier, wantExpiry := redeemFields(t, dotnetResp.body)
	if gotTier != wantTier {
		t.Errorf("tier: go=%q dotnet=%q", gotTier, wantTier)
	}
	if gotTier != "Personal" {
		t.Errorf("redeemed tier: got %q, want Personal", gotTier)
	}
	if gotExpiry.IsZero() || wantExpiry.IsZero() {
		t.Errorf("expiresAt must be present and parseable: go=%v dotnet=%v", gotExpiry, wantExpiry)
	}
}

// adminExchange is one X-Admin-Key request.
type adminExchange struct {
	status      int
	contentType string
	body        []byte
}

func adminRequest(t *testing.T, client *http.Client, base, method, path, key, body string) adminExchange {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, reader)
	if err != nil {
		t.Fatalf("new request %s %s: %v", method, path, err)
	}
	req.Header.Set("X-Admin-Key", key)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		t.Fatalf("read body %s %s: %v", method, path, err)
	}
	return adminExchange{status: resp.StatusCode, contentType: resp.Header.Get("Content-Type"), body: raw}
}

// diffAdmin runs the same X-Admin-Key request against both APIs and asserts
// status, content type, and JSON/raw body match (the .NET response is the source
// of truth).
func diffAdmin(t *testing.T, client *http.Client, dotnetURL, goURL, method, path, key, body string) {
	t.Helper()

	want := adminRequest(t, client, dotnetURL, method, path, key, body)
	got := adminRequest(t, client, goURL, method, path, key, body)

	if got.status != want.status {
		t.Errorf("%s %s status: go=%d dotnet=%d (go body %s)", method, path, got.status, want.status, got.body)
	}
	if got.contentType != want.contentType {
		t.Errorf("%s %s content-type: go=%q dotnet=%q", method, path, got.contentType, want.contentType)
	}
	if len(want.body) == 0 || len(got.body) == 0 {
		if !bytes.Equal(got.body, want.body) {
			t.Errorf("%s %s body: go=%q dotnet=%q", method, path, got.body, want.body)
		}
		return
	}
	if !jsonEqual(t, got.body, want.body) {
		t.Errorf("%s %s body: go=%s dotnet=%s", method, path, got.body, want.body)
	}
}

// adminKey returns the shared admin key, skipping the test when it is not set
// (so plain local runs stay green; the value is wired in CI).
func adminKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv("ADMIN_API_KEY")
	if key == "" {
		t.Skip("ADMIN_API_KEY not set — admin/offer-code authed contract scenarios run in CI")
	}
	return key
}

// integrationEmail returns the integration user's email (its Auth0 username),
// skipping when unset.
func integrationEmail(t *testing.T) string {
	t.Helper()
	email := os.Getenv("INTEGRATION_TEST_USERNAME")
	if email == "" {
		t.Skip("INTEGRATION_TEST_USERNAME not set — admin-authed contract scenarios run in CI")
	}
	return email
}

func redeemBody(code string) string {
	return fmt.Sprintf(`{"code":%q}`, code)
}

// generateCodes mints count display-formatted codes for a paid tier via the
// admin endpoint and returns them (Normalize accepts the display format).
func generateCodes(t *testing.T, client *http.Client, base, key string, count int, tier string, durationDays int) []string {
	t.Helper()
	body := fmt.Sprintf(`{"count":%d,"tier":%q,"durationDays":%d}`, count, tier, durationDays)
	resp := adminRequest(t, client, base, http.MethodPost, "/v1/admin/offer-codes", key, body)
	if resp.status != http.StatusOK {
		t.Fatalf("generate codes: status %d body %s", resp.status, resp.body)
	}
	var codes []string
	for _, line := range strings.Split(strings.TrimSpace(string(resp.body)), "\n") {
		if s := strings.TrimSpace(line); s != "" {
			codes = append(codes, s)
		}
	}
	return codes
}

// currentTier reads the integration user's current tier from the admin list.
func currentTier(t *testing.T, client *http.Client, base, key, email string) string {
	t.Helper()
	resp := adminRequest(t, client, base, http.MethodGet, "/v1/admin/users?pageSize=100&search="+url.QueryEscape(email), key, "")
	if resp.status != http.StatusOK {
		t.Fatalf("read current tier: status %d body %s", resp.status, resp.body)
	}
	var doc struct {
		Items []struct {
			Email *string `json:"email"`
			Tier  string  `json:"tier"`
		} `json:"items"`
	}
	if err := json.Unmarshal(resp.body, &doc); err != nil {
		t.Fatalf("decode users: %v (%s)", err, resp.body)
	}
	for _, it := range doc.Items {
		if it.Email != nil && strings.EqualFold(*it.Email, email) {
			return it.Tier
		}
	}
	t.Fatalf("integration user %q not found in admin list (%s)", email, resp.body)
	return ""
}

// grantTier sets the user's tier by email via the admin endpoint.
func grantTier(t *testing.T, client *http.Client, base, key, email, tier string) {
	t.Helper()
	body := fmt.Sprintf(`{"email":%q,"tier":%q}`, email, tier)
	if r := adminRequest(t, client, base, http.MethodPut, "/v1/admin/subscriptions", key, body); r.status != http.StatusOK {
		t.Fatalf("grant %s to %s: status %d body %s", tier, email, r.status, r.body)
	}
}

// redeemFields extracts the tier and parsed expiresAt from a redeem response.
func redeemFields(t *testing.T, body []byte) (string, time.Time) {
	t.Helper()
	var doc struct {
		Tier      string `json:"tier"`
		ExpiresAt string `json:"expiresAt"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("decode redeem response: %v (%s)", err, body)
	}
	ts, err := time.Parse(time.RFC3339, doc.ExpiresAt)
	if err != nil {
		t.Fatalf("parse expiresAt %q: %v", doc.ExpiresAt, err)
	}
	return doc.Tier, ts
}

// assertOneOfferCodeLine fails unless body is exactly one non-empty line.
func assertOneOfferCodeLine(t *testing.T, who string, body []byte) {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) != 1 || strings.TrimSpace(lines[0]) == "" {
		t.Errorf("%s: generate must return exactly one code line, got %q", who, body)
	}
}

// adminUserItemsSorted decodes an admin users page and returns its items array
// sorted by userId, re-marshalled — so the cross-partition, order-undefined list
// can be diffed for content rather than order (jsonEqual normalises key order).
func adminUserItemsSorted(t *testing.T, body []byte) []byte {
	t.Helper()
	var doc struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("decode admin users: %v (%s)", err, body)
	}
	sort.Slice(doc.Items, func(i, j int) bool {
		return userIDOf(doc.Items[i]) < userIDOf(doc.Items[j])
	})
	out, err := json.Marshal(doc.Items)
	if err != nil {
		t.Fatalf("marshal items: %v", err)
	}
	return out
}

func userIDOf(m map[string]any) string {
	if v, ok := m["userId"].(string); ok {
		return v
	}
	return ""
}
