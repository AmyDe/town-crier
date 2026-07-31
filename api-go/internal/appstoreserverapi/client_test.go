package appstoreserverapi

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// newTestKeyPEM generates a P-256 ECDSA key encoded as a PKCS8 PEM block, the
// same shape Apple ships a .p8 auth key in — mirrors internal/apns/jwt_test.go.
func newTestKeyPEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal pkcs8: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

func decodeSegment(t *testing.T, seg string) []byte {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		t.Fatalf("base64url decode %q: %v", seg, err)
	}
	return b
}

const (
	testKeyID    = "ABCDE12345"
	testIssuerID = "57246542-96fe-1a63-e053-0824d011072a"
	testBundleID = "uk.towncrierapp.mobile"
)

func newTestOptions(pemBytes []byte, environment string) Options {
	return Options{
		SigningKey:  string(pemBytes),
		KeyID:       testKeyID,
		IssuerID:    testIssuerID,
		BundleID:    testBundleID,
		Environment: environment,
	}
}

// --- JWT shape ---

// TestClient_FetchNotificationHistory_MintsValidJWT decodes the
// Authorization: Bearer header server-side and asserts the header/claims
// shape the spec pins: alg=ES256/kid=<KeyID>, iss=<IssuerID>,
// exp<=iat+3600, aud=appstoreconnect-v1, bid=<BundleID>.
func TestClient_FetchNotificationHistory_MintsValidJWT(t *testing.T) {
	t.Parallel()
	pemBytes := newTestKeyPEM(t)

	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		writeHistoryResponse(w, historyResponse{})
	}))
	defer srv.Close()

	now := time.Unix(1_800_000_000, 0).UTC()
	client, err := newClientWithBaseURL(newTestOptions(pemBytes, "Production"), srv.URL, srv.Client(), func() time.Time { return now })
	if err != nil {
		t.Fatalf("newClientWithBaseURL: %v", err)
	}

	if _, err := client.FetchNotificationHistory(context.Background(), now.Add(-time.Hour), now); err != nil {
		t.Fatalf("FetchNotificationHistory: %v", err)
	}

	if !strings.HasPrefix(capturedAuth, "Bearer ") {
		t.Fatalf("Authorization header = %q, want Bearer prefix", capturedAuth)
	}
	token := strings.TrimPrefix(capturedAuth, "Bearer ")
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token must have 3 dot-separated parts, got %d", len(parts))
	}

	var header jwtHeader
	if err := json.Unmarshal(decodeSegment(t, parts[0]), &header); err != nil {
		t.Fatalf("decode header: %v", err)
	}
	if header.Alg != "ES256" {
		t.Errorf("header.alg = %q, want ES256", header.Alg)
	}
	if header.Kid != testKeyID {
		t.Errorf("header.kid = %q, want %q", header.Kid, testKeyID)
	}
	if header.Typ != "JWT" {
		t.Errorf("header.typ = %q, want JWT", header.Typ)
	}

	var claims jwtClaims
	if err := json.Unmarshal(decodeSegment(t, parts[1]), &claims); err != nil {
		t.Fatalf("decode claims: %v", err)
	}
	if claims.Iss != testIssuerID {
		t.Errorf("claims.iss = %q, want %q", claims.Iss, testIssuerID)
	}
	if claims.Iat != now.Unix() {
		t.Errorf("claims.iat = %d, want %d", claims.Iat, now.Unix())
	}
	if claims.Exp > claims.Iat+3600 {
		t.Errorf("claims.exp = %d, iat = %d: exp exceeds iat+3600", claims.Exp, claims.Iat)
	}
	if claims.Exp <= claims.Iat {
		t.Errorf("claims.exp = %d, iat = %d: exp must be after iat", claims.Exp, claims.Iat)
	}
	if claims.Aud != "appstoreconnect-v1" {
		t.Errorf("claims.aud = %q, want appstoreconnect-v1", claims.Aud)
	}
	if claims.Bid != testBundleID {
		t.Errorf("claims.bid = %q, want %q", claims.Bid, testBundleID)
	}
}

// --- base URL selection ---

func TestClient_BaseURLSelection(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		env  string
		want string
	}{
		{"production", "Production", productionBaseURL},
		{"sandbox", "Sandbox", sandboxBaseURL},
		{"sandbox case-insensitive", "sandbox", sandboxBaseURL},
		{"unset defaults to production", "", productionBaseURL},
		{"unrecognized defaults to production", "Staging", productionBaseURL},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := newTestOptions(nil, tc.env).baseURL()
			if got != tc.want {
				t.Errorf("baseURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

// --- single-page fetch ---

func writeHistoryResponse(w http.ResponseWriter, resp historyResponse) {
	w.Header().Set("Content-Type", "application/json")
	body, _ := json.Marshal(resp)
	_, _ = w.Write(body)
}

func TestClient_FetchNotificationHistory_SinglePage(t *testing.T) {
	t.Parallel()
	pemBytes := newTestKeyPEM(t)

	var capturedBody historyRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		writeHistoryResponse(w, historyResponse{
			NotificationHistory: []historyResponseItem{
				{SignedPayload: "jws-1"},
				{SignedPayload: "jws-2"},
			},
			HasMore: false,
		})
	}))
	defer srv.Close()

	now := time.Unix(1_800_000_000, 0).UTC()
	client, err := newClientWithBaseURL(newTestOptions(pemBytes, "Production"), srv.URL, srv.Client(), func() time.Time { return now })
	if err != nil {
		t.Fatalf("newClientWithBaseURL: %v", err)
	}

	start := now.Add(-24 * time.Hour)
	items, err := client.FetchNotificationHistory(context.Background(), start, now)
	if err != nil {
		t.Fatalf("FetchNotificationHistory: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %v, want 2 entries", items)
	}
	if items[0].SignedPayload != "jws-1" || items[1].SignedPayload != "jws-2" {
		t.Errorf("items = %+v, want [jws-1 jws-2]", items)
	}
	if capturedBody.StartDate != start.UnixMilli() {
		t.Errorf("startDate = %d, want %d", capturedBody.StartDate, start.UnixMilli())
	}
	if capturedBody.EndDate != now.UnixMilli() {
		t.Errorf("endDate = %d, want %d", capturedBody.EndDate, now.UnixMilli())
	}
	if capturedBody.PaginationToken != "" {
		t.Errorf("paginationToken = %q, want empty on first page", capturedBody.PaginationToken)
	}
}

// --- multi-page fetch ---

func TestClient_FetchNotificationHistory_MultiPageThreadsToken(t *testing.T) {
	t.Parallel()
	pemBytes := newTestKeyPEM(t)

	var capturedTokens []string
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req historyRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		capturedTokens = append(capturedTokens, req.PaginationToken)
		callCount++
		switch callCount {
		case 1:
			writeHistoryResponse(w, historyResponse{
				NotificationHistory: []historyResponseItem{{SignedPayload: "page1-a"}},
				HasMore:             true,
				PaginationToken:     "token-2",
			})
		case 2:
			writeHistoryResponse(w, historyResponse{
				NotificationHistory: []historyResponseItem{{SignedPayload: "page2-a"}},
				HasMore:             true,
				PaginationToken:     "token-3",
			})
		default:
			writeHistoryResponse(w, historyResponse{
				NotificationHistory: []historyResponseItem{{SignedPayload: "page3-a"}},
				HasMore:             false,
			})
		}
	}))
	defer srv.Close()

	now := time.Unix(1_800_000_000, 0).UTC()
	client, err := newClientWithBaseURL(newTestOptions(pemBytes, "Production"), srv.URL, srv.Client(), func() time.Time { return now })
	if err != nil {
		t.Fatalf("newClientWithBaseURL: %v", err)
	}

	items, err := client.FetchNotificationHistory(context.Background(), now.Add(-time.Hour), now)
	if err != nil {
		t.Fatalf("FetchNotificationHistory: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("items = %v, want 3 entries across 3 pages", items)
	}
	wantTokens := []string{"", "token-2", "token-3"}
	if len(capturedTokens) != len(wantTokens) {
		t.Fatalf("capturedTokens = %v, want %v", capturedTokens, wantTokens)
	}
	for i, want := range wantTokens {
		if capturedTokens[i] != want {
			t.Errorf("capturedTokens[%d] = %q, want %q", i, capturedTokens[i], want)
		}
	}
}

// --- 429 handling ---

func TestClient_FetchNotificationHistory_RetriesOn429WithSecondsRetryAfter(t *testing.T) {
	t.Parallel()
	pemBytes := newTestKeyPEM(t)

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writeHistoryResponse(w, historyResponse{})
	}))
	defer srv.Close()

	now := time.Unix(1_800_000_000, 0).UTC()
	client, err := newClientWithBaseURL(newTestOptions(pemBytes, "Production"), srv.URL, srv.Client(), func() time.Time { return now })
	if err != nil {
		t.Fatalf("newClientWithBaseURL: %v", err)
	}

	if _, err := client.FetchNotificationHistory(context.Background(), now.Add(-time.Hour), now); err != nil {
		t.Fatalf("FetchNotificationHistory: %v", err)
	}
	if callCount != 2 {
		t.Errorf("callCount = %d, want 2 (one 429 then success)", callCount)
	}
}

func TestClient_FetchNotificationHistory_RetriesOn429WithHTTPDateRetryAfter(t *testing.T) {
	t.Parallel()
	pemBytes := newTestKeyPEM(t)

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// An HTTP-date Retry-After effectively "now" so the test does not
			// actually sleep.
			w.Header().Set("Retry-After", time.Now().UTC().Format(http.TimeFormat))
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writeHistoryResponse(w, historyResponse{})
	}))
	defer srv.Close()

	now := time.Unix(1_800_000_000, 0).UTC()
	client, err := newClientWithBaseURL(newTestOptions(pemBytes, "Production"), srv.URL, srv.Client(), func() time.Time { return now })
	if err != nil {
		t.Fatalf("newClientWithBaseURL: %v", err)
	}

	if _, err := client.FetchNotificationHistory(context.Background(), now.Add(-time.Hour), now); err != nil {
		t.Fatalf("FetchNotificationHistory: %v", err)
	}
	if callCount != 2 {
		t.Errorf("callCount = %d, want 2 (one 429 then success)", callCount)
	}
}

func TestClient_FetchNotificationHistory_RetriesExhaustedReturnsError(t *testing.T) {
	t.Parallel()
	pemBytes := newTestKeyPEM(t)

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	now := time.Unix(1_800_000_000, 0).UTC()
	client, err := newClientWithBaseURL(newTestOptions(pemBytes, "Production"), srv.URL, srv.Client(), func() time.Time { return now })
	if err != nil {
		t.Fatalf("newClientWithBaseURL: %v", err)
	}

	_, err = client.FetchNotificationHistory(context.Background(), now.Add(-time.Hour), now)
	if err == nil {
		t.Fatal("FetchNotificationHistory: want error after exhausting retries, got nil")
	}
	if callCount < 2 {
		t.Errorf("callCount = %d, want at least 2 attempts before giving up", callCount)
	}
	if callCount > maxAttempts {
		t.Errorf("callCount = %d, want at most maxAttempts=%d", callCount, maxAttempts)
	}
}

// --- non-429 error ---

func TestClient_FetchNotificationHistory_NonRetryableStatusFailsImmediately(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		status int
	}{
		{"unauthorized", http.StatusUnauthorized},
		{"internal server error", http.StatusInternalServerError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pemBytes := newTestKeyPEM(t)

			callCount := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callCount++
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()

			now := time.Unix(1_800_000_000, 0).UTC()
			client, err := newClientWithBaseURL(newTestOptions(pemBytes, "Production"), srv.URL, srv.Client(), func() time.Time { return now })
			if err != nil {
				t.Fatalf("newClientWithBaseURL: %v", err)
			}

			_, err = client.FetchNotificationHistory(context.Background(), now.Add(-time.Hour), now)
			if err == nil {
				t.Fatalf("FetchNotificationHistory: want error for status %d, got nil", tc.status)
			}
			if callCount != 1 {
				t.Errorf("callCount = %d, want 1 (no retry on a non-429 error)", callCount)
			}
		})
	}
}

// --- max-page cap guard ---

func TestClient_FetchNotificationHistory_MaxPageCapGuardsAgainstInfiniteLoop(t *testing.T) {
	t.Parallel()
	pemBytes := newTestKeyPEM(t)

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		writeHistoryResponse(w, historyResponse{
			NotificationHistory: []historyResponseItem{{SignedPayload: fmt.Sprintf("jws-%d", callCount)}},
			HasMore:             true, // never terminates on its own
			PaginationToken:     strconv.Itoa(callCount),
		})
	}))
	defer srv.Close()

	now := time.Unix(1_800_000_000, 0).UTC()
	client, err := newClientWithBaseURL(newTestOptions(pemBytes, "Production"), srv.URL, srv.Client(), func() time.Time { return now })
	if err != nil {
		t.Fatalf("newClientWithBaseURL: %v", err)
	}

	_, err = client.FetchNotificationHistory(context.Background(), now.Add(-time.Hour), now)
	if err == nil {
		t.Fatal("FetchNotificationHistory: want error when hasMore never terminates, got nil")
	}
	if callCount != maxHistoryPages {
		t.Errorf("callCount = %d, want exactly maxHistoryPages=%d (defensive cap fired)", callCount, maxHistoryPages)
	}
}

// --- options validation ---

func TestNewClient_RejectsIncompleteOptions(t *testing.T) {
	t.Parallel()
	pemBytes := newTestKeyPEM(t)
	tests := []struct {
		name string
		opts Options
	}{
		{"missing key id", Options{SigningKey: string(pemBytes), IssuerID: testIssuerID, BundleID: testBundleID}},
		{"missing issuer id", Options{SigningKey: string(pemBytes), KeyID: testKeyID, BundleID: testBundleID}},
		{"missing bundle id", Options{SigningKey: string(pemBytes), KeyID: testKeyID, IssuerID: testIssuerID}},
		{"missing signing key", Options{KeyID: testKeyID, IssuerID: testIssuerID, BundleID: testBundleID}},
		{"garbage signing key", Options{SigningKey: "not a pem", KeyID: testKeyID, IssuerID: testIssuerID, BundleID: testBundleID}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewClient(tc.opts, time.Now); err == nil {
				t.Error("NewClient: want error, got nil")
			}
		})
	}
}
