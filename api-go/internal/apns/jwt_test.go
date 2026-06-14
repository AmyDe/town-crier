package apns

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"strings"
	"testing"
	"time"
)

// newTestKeyPEM generates a P-256 ECDSA key encoded as a PKCS8 PEM block, the
// same shape Apple ships a .p8 auth key in. Returned alongside the key so a
// test can verify the JWT signature against the public half.
func newTestKeyPEM(t *testing.T) (pemBytes []byte, pub *ecdsa.PublicKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
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

func decodeSegment(t *testing.T, seg string) []byte {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		t.Fatalf("base64url decode %q: %v", seg, err)
	}
	return b
}

func TestJWTProvider_MintsValidES256Token(t *testing.T) {
	t.Parallel()

	pemBytes, _ := newTestKeyPEM(t)
	now := time.Unix(1_700_000_000, 0).UTC()

	provider, err := newJWTProvider(pemBytes, "L2J5PQASN5", "4574VQ7N2X", func() time.Time { return now })
	if err != nil {
		t.Fatalf("newJWTProvider: %v", err)
	}

	token, err := provider.current()
	if err != nil {
		t.Fatalf("current: %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token must have 3 dot-separated parts, got %d", len(parts))
	}

	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(decodeSegment(t, parts[0]), &header); err != nil {
		t.Fatalf("decode header: %v", err)
	}
	if header.Alg != "ES256" {
		t.Errorf("alg = %q, want ES256", header.Alg)
	}
	if header.Kid != "L2J5PQASN5" {
		t.Errorf("kid = %q, want L2J5PQASN5", header.Kid)
	}

	var claims struct {
		Iss string `json:"iss"`
		Iat int64  `json:"iat"`
	}
	if err := json.Unmarshal(decodeSegment(t, parts[1]), &claims); err != nil {
		t.Fatalf("decode claims: %v", err)
	}
	if claims.Iss != "4574VQ7N2X" {
		t.Errorf("iss = %q, want 4574VQ7N2X", claims.Iss)
	}
	if claims.Iat != now.Unix() {
		t.Errorf("iat = %d, want %d", claims.Iat, now.Unix())
	}

	// The signature must be 64 raw bytes (r||s), the JWS ES256 form — never the
	// ASN.1 DER form crypto/ecdsa.Sign emits by default.
	sig := decodeSegment(t, parts[2])
	if len(sig) != 64 {
		t.Fatalf("signature = %d bytes, want 64 (JWS r||s form)", len(sig))
	}
}

func TestJWTProvider_SignatureVerifiesAgainstPublicKey(t *testing.T) {
	t.Parallel()

	pemBytes, pub := newTestKeyPEM(t)
	now := time.Unix(1_700_000_000, 0).UTC()

	provider, err := newJWTProvider(pemBytes, "L2J5PQASN5", "4574VQ7N2X", func() time.Time { return now })
	if err != nil {
		t.Fatalf("newJWTProvider: %v", err)
	}

	token, err := provider.current()
	if err != nil {
		t.Fatalf("current: %v", err)
	}

	parts := strings.Split(token, ".")
	signingInput := parts[0] + "." + parts[1]
	sig := decodeSegment(t, parts[2])

	digest := sha256Sum([]byte(signingInput))
	r := bigIntFromBytes(sig[:32])
	s := bigIntFromBytes(sig[32:])
	if !ecdsa.Verify(pub, digest[:], r, s) {
		t.Fatal("signature failed to verify against the public key")
	}
}

func TestJWTProvider_CachesUntilRefreshInterval(t *testing.T) {
	t.Parallel()

	pemBytes, _ := newTestKeyPEM(t)
	current := time.Unix(1_700_000_000, 0).UTC()
	clock := func() time.Time { return current }

	provider, err := newJWTProvider(pemBytes, "L2J5PQASN5", "4574VQ7N2X", clock)
	if err != nil {
		t.Fatalf("newJWTProvider: %v", err)
	}

	first, err := provider.current()
	if err != nil {
		t.Fatalf("first current: %v", err)
	}

	// 49 minutes on: still inside the 50-minute refresh window, so the same
	// cached token comes back.
	current = current.Add(49 * time.Minute)
	cached, err := provider.current()
	if err != nil {
		t.Fatalf("cached current: %v", err)
	}
	if cached != first {
		t.Error("token should be cached within the refresh interval")
	}

	// 51 minutes on: past the window, a fresh token is minted.
	current = current.Add(2 * time.Minute)
	fresh, err := provider.current()
	if err != nil {
		t.Fatalf("fresh current: %v", err)
	}
	if fresh == first {
		t.Error("token should be re-minted after the refresh interval")
	}
}

func TestJWTProvider_InvalidateForcesRemint(t *testing.T) {
	t.Parallel()

	pemBytes, _ := newTestKeyPEM(t)
	current := time.Unix(1_700_000_000, 0).UTC()
	// Advance the clock by one second per call so a re-mint produces a token
	// with a different iat (and therefore different bytes).
	clock := func() time.Time {
		current = current.Add(time.Second)
		return current
	}

	provider, err := newJWTProvider(pemBytes, "L2J5PQASN5", "4574VQ7N2X", clock)
	if err != nil {
		t.Fatalf("newJWTProvider: %v", err)
	}

	first, err := provider.current()
	if err != nil {
		t.Fatalf("first current: %v", err)
	}

	provider.invalidate()

	second, err := provider.current()
	if err != nil {
		t.Fatalf("second current: %v", err)
	}
	if second == first {
		t.Error("invalidate should force a fresh mint on the next current call")
	}
}

func TestNewJWTProvider_RejectsBadKey(t *testing.T) {
	t.Parallel()

	_, err := newJWTProvider([]byte("not a pem key"), "L2J5PQASN5", "4574VQ7N2X", time.Now)
	if err == nil {
		t.Fatal("expected an error for a malformed key, got nil")
	}
}
