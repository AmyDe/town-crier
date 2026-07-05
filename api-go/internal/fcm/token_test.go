package fcm

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestKeyPEM generates an RSA-2048 key encoded as a PKCS8 PEM block, the same
// shape a Google service-account key carries in its private_key field. Returned
// alongside the public half so a test can verify the assertion signature.
func newTestKeyPEM(t *testing.T) (pemBytes []byte, pub *rsa.PublicKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal pkcs8: %v", err)
	}
	pemBytes = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	return pemBytes, &key.PublicKey
}

// newTestServiceAccountJSON builds a service-account key JSON pointing its token
// endpoint at tokenURI, with the given PEM private key.
func newTestServiceAccountJSON(t *testing.T, tokenURI string, pemBytes []byte) string {
	t.Helper()
	sa := map[string]string{
		"type":         "service_account",
		"project_id":   "town-crier-test",
		"client_email": "fcm@town-crier-test.iam.gserviceaccount.com",
		"private_key":  string(pemBytes),
		"token_uri":    tokenURI,
	}
	raw, err := json.Marshal(sa)
	if err != nil {
		t.Fatalf("marshal service account: %v", err)
	}
	return string(raw)
}

func fixedClock(unix int64) func() time.Time {
	return func() time.Time { return time.Unix(unix, 0).UTC() }
}

func TestTokenProvider_MintsValidRS256Assertion(t *testing.T) {
	t.Parallel()
	pemBytes, pub := newTestKeyPEM(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	saJSON := newTestServiceAccountJSON(t, "https://oauth2.googleapis.com/token", pemBytes)

	provider, err := newTokenProvider([]byte(saJSON), &http.Client{}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newTokenProvider: %v", err)
	}

	assertion, err := provider.mintAssertion(now)
	if err != nil {
		t.Fatalf("mintAssertion: %v", err)
	}

	parts := strings.Split(assertion, ".")
	if len(parts) != 3 {
		t.Fatalf("assertion must have 3 dot-separated parts, got %d", len(parts))
	}

	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(decodeSegment(t, parts[0]), &header); err != nil {
		t.Fatalf("decode header: %v", err)
	}
	if header.Alg != "RS256" {
		t.Errorf("alg = %q, want RS256", header.Alg)
	}

	var claims struct {
		Iss   string `json:"iss"`
		Scope string `json:"scope"`
		Aud   string `json:"aud"`
		Iat   int64  `json:"iat"`
		Exp   int64  `json:"exp"`
	}
	if err := json.Unmarshal(decodeSegment(t, parts[1]), &claims); err != nil {
		t.Fatalf("decode claims: %v", err)
	}
	if claims.Iss != "fcm@town-crier-test.iam.gserviceaccount.com" {
		t.Errorf("iss = %q", claims.Iss)
	}
	if claims.Scope != firebaseMessagingScope {
		t.Errorf("scope = %q, want %q", claims.Scope, firebaseMessagingScope)
	}
	if claims.Aud != "https://oauth2.googleapis.com/token" {
		t.Errorf("aud = %q", claims.Aud)
	}
	if claims.Iat != now.Unix() {
		t.Errorf("iat = %d, want %d", claims.Iat, now.Unix())
	}
	if claims.Exp != now.Add(time.Hour).Unix() {
		t.Errorf("exp = %d, want %d", claims.Exp, now.Add(time.Hour).Unix())
	}

	// The signature must verify against the public key under RS256.
	signingInput := parts[0] + "." + parts[1]
	digest := sha256.Sum256([]byte(signingInput))
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], decodeSegment(t, parts[2])); err != nil {
		t.Fatalf("signature failed to verify: %v", err)
	}
}

func TestTokenProvider_ExchangesAssertionForAccessToken(t *testing.T) {
	t.Parallel()
	pemBytes, _ := newTestKeyPEM(t)

	var gotGrant, gotAssertion, gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotGrant = r.PostForm.Get("grant_type")
		gotAssertion = r.PostForm.Get("assertion")
		gotContentType = r.Header.Get("content-type")
		_, _ = w.Write([]byte(`{"access_token":"ya29.test","expires_in":3599,"token_type":"Bearer"}`))
	}))
	defer srv.Close()

	saJSON := newTestServiceAccountJSON(t, srv.URL, pemBytes)
	provider, err := newTokenProvider([]byte(saJSON), srv.Client(), fixedClock(1_700_000_000))
	if err != nil {
		t.Fatalf("newTokenProvider: %v", err)
	}

	token, err := provider.current(context.Background())
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	if token != "ya29.test" {
		t.Errorf("token = %q, want ya29.test", token)
	}
	if gotGrant != jwtBearerGrant {
		t.Errorf("grant_type = %q, want %q", gotGrant, jwtBearerGrant)
	}
	if gotAssertion == "" || len(strings.Split(gotAssertion, ".")) != 3 {
		t.Errorf("assertion = %q, want a 3-part JWT", gotAssertion)
	}
	if gotContentType != "application/x-www-form-urlencoded" {
		t.Errorf("content-type = %q", gotContentType)
	}
}

func TestTokenProvider_CachesUntilExpiry(t *testing.T) {
	t.Parallel()
	pemBytes, _ := newTestKeyPEM(t)

	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"access_token":"ya29.test","expires_in":3599,"token_type":"Bearer"}`))
	}))
	defer srv.Close()

	current := time.Unix(1_700_000_000, 0).UTC()
	saJSON := newTestServiceAccountJSON(t, srv.URL, pemBytes)
	provider, err := newTokenProvider([]byte(saJSON), srv.Client(), func() time.Time { return current })
	if err != nil {
		t.Fatalf("newTokenProvider: %v", err)
	}

	if _, err := provider.current(context.Background()); err != nil {
		t.Fatalf("first current: %v", err)
	}
	// Well inside the token lifetime (3599s - 60s margin): the cached token is reused.
	current = current.Add(30 * time.Minute)
	if _, err := provider.current(context.Background()); err != nil {
		t.Fatalf("second current: %v", err)
	}
	if calls != 1 {
		t.Errorf("token endpoint calls = %d, want 1 (cached)", calls)
	}

	// Past the effective expiry: a fresh token is fetched.
	current = current.Add(time.Hour)
	if _, err := provider.current(context.Background()); err != nil {
		t.Fatalf("third current: %v", err)
	}
	if calls != 2 {
		t.Errorf("token endpoint calls = %d, want 2 (re-fetched after expiry)", calls)
	}
}

func TestNewTokenProvider_RejectsBadKey(t *testing.T) {
	t.Parallel()
	saJSON := fmt.Sprintf(`{"client_email":"x@y.iam","token_uri":"%s","private_key":"not a pem"}`, "https://oauth2.googleapis.com/token")
	_, err := newTokenProvider([]byte(saJSON), &http.Client{}, time.Now)
	if !errors.Is(err, ErrInvalidServiceAccount) {
		t.Fatalf("err = %v, want ErrInvalidServiceAccount", err)
	}
}

func TestNewTokenProvider_RejectsMalformedJSON(t *testing.T) {
	t.Parallel()
	_, err := newTokenProvider([]byte("{not json"), &http.Client{}, time.Now)
	if !errors.Is(err, ErrInvalidServiceAccount) {
		t.Fatalf("err = %v, want ErrInvalidServiceAccount", err)
	}
}

func decodeSegment(t *testing.T, seg string) []byte {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		t.Fatalf("base64url decode %q: %v", seg, err)
	}
	return b
}
