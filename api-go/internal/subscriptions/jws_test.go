package subscriptions

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"testing"
	"time"
)

const testPayloadJSON = `{"transactionId":"txn-1","bundleId":"uk.towncrierapp.mobile"}`

func TestJWSVerifier_ValidChainAndSignature(t *testing.T) {
	t.Parallel()
	b := newTestJWSBuilder(t, time.Time{}, time.Time{})
	v := newVerifier(t, b.root)

	payload, err := v.VerifyAndDecode(b.sign(t, testPayloadJSON))
	if err != nil {
		t.Fatalf("VerifyAndDecode: %v", err)
	}
	if payload != testPayloadJSON {
		t.Errorf("payload = %q, want %q", payload, testPayloadJSON)
	}
}

func TestJWSVerifier_TamperedSignature(t *testing.T) {
	t.Parallel()
	b := newTestJWSBuilder(t, time.Time{}, time.Time{})
	v := newVerifier(t, b.root)

	_, err := v.VerifyAndDecode(b.signTampered(t, testPayloadJSON))
	requireJWSError(t, err)
}

func TestJWSVerifier_ChainDoesNotReachTrustedRoot(t *testing.T) {
	t.Parallel()
	signing := newTestJWSBuilder(t, time.Time{}, time.Time{})
	unrelated := newTestJWSBuilder(t, time.Time{}, time.Time{})
	v := newVerifier(t, unrelated.root)

	_, err := v.VerifyAndDecode(signing.sign(t, testPayloadJSON))
	requireJWSError(t, err)
}

func TestJWSVerifier_LeafExpired(t *testing.T) {
	t.Parallel()
	now := time.Now()
	b := newTestJWSBuilder(t, now.AddDate(-2, 0, 0), now.AddDate(-1, 0, 0))
	v := newVerifier(t, b.root)

	_, err := v.VerifyAndDecode(b.sign(t, testPayloadJSON))
	requireJWSError(t, err)
}

func TestJWSVerifier_NotThreeParts(t *testing.T) {
	t.Parallel()
	b := newTestJWSBuilder(t, time.Time{}, time.Time{})
	v := newVerifier(t, b.root)

	_, err := v.VerifyAndDecode("not-a-jws")
	requireJWSError(t, err)
}

func TestJWSVerifier_HeaderHasNoChain(t *testing.T) {
	t.Parallel()
	b := newTestJWSBuilder(t, time.Time{}, time.Time{})
	v := newVerifier(t, b.root)

	// header {"alg":"ES256"}, payload {"a":1}, signature "sig" — structurally a
	// three-part JWS but with no x5c chain.
	_, err := v.VerifyAndDecode("eyJhbGciOiJFUzI1NiJ9.eyJhIjoxfQ.c2ln")
	requireJWSError(t, err)
}

func TestJWSVerifier_EmptyPayload(t *testing.T) {
	t.Parallel()
	b := newTestJWSBuilder(t, time.Time{}, time.Time{})
	v := newVerifier(t, b.root)

	_, err := v.VerifyAndDecode("   ")
	requireJWSError(t, err)
}

func TestJWSVerifier_UnsupportedAlgorithm(t *testing.T) {
	t.Parallel()
	b := newTestJWSBuilder(t, time.Time{}, time.Time{})
	v := newVerifier(t, b.root)

	// A well-formed JWS whose header declares RS256 must be rejected: Apple uses
	// ES256 exclusively.
	_, err := v.VerifyAndDecode(b.signWithAlg(t, testPayloadJSON, "RS256"))
	requireJWSError(t, err)
}

func TestLoadAppleRootCertificates(t *testing.T) {
	t.Parallel()
	roots, err := LoadAppleRootCertificates()
	if err != nil {
		t.Fatalf("LoadAppleRootCertificates: %v", err)
	}
	if len(roots) != 1 {
		t.Fatalf("got %d roots, want 1", len(roots))
	}
	if got := roots[0].Subject.CommonName; got != "Apple Root CA - G3" {
		t.Errorf("root CN = %q, want %q", got, "Apple Root CA - G3")
	}
}

func newVerifier(t *testing.T, root *x509.Certificate) *JWSVerifier {
	t.Helper()
	v, err := NewJWSVerifier([]*x509.Certificate{root}, time.Now)
	if err != nil {
		t.Fatalf("NewJWSVerifier: %v", err)
	}
	return v
}

func requireJWSError(t *testing.T, err error) {
	t.Helper()
	var je *JWSVerificationError
	if !errors.As(err, &je) {
		t.Fatalf("want *JWSVerificationError, got %T (%v)", err, err)
	}
}

// testJWSBuilder builds a self-contained EC certificate chain
// (root -> intermediate -> leaf) and signs an ES256 JWS the way Apple's
// StoreKit infrastructure does: an x5c header carrying the chain (leaf first)
// and an ES256 signature over header.payload. Mirrors the .NET TestJwsBuilder.
type testJWSBuilder struct {
	root         *x509.Certificate
	intermediate *x509.Certificate
	leaf         *x509.Certificate
	leafKey      *ecdsa.PrivateKey
}

func newTestJWSBuilder(t *testing.T, leafNotBefore, leafNotAfter time.Time) *testJWSBuilder {
	t.Helper()
	now := time.Now()
	if leafNotBefore.IsZero() {
		leafNotBefore = now.AddDate(0, 0, -1)
	}
	if leafNotAfter.IsZero() {
		leafNotAfter = now.AddDate(1, 0, 0)
	}

	rootKey := genKey(t)
	rootTmpl := caTemplate(t, "Test Apple Root CA", now.AddDate(-1, 0, 0), now.AddDate(10, 0, 0))
	root := mkCert(t, rootTmpl, rootTmpl, &rootKey.PublicKey, rootKey)

	interKey := genKey(t)
	interTmpl := caTemplate(t, "Test Apple Intermediate CA", now.AddDate(-1, 0, 0), now.AddDate(5, 0, 0))
	inter := mkCert(t, interTmpl, root, &interKey.PublicKey, rootKey)

	leafKey := genKey(t)
	leafTmpl := &x509.Certificate{
		SerialNumber: serial(t),
		Subject:      pkix.Name{CommonName: "Test Apple Leaf"},
		NotBefore:    leafNotBefore,
		NotAfter:     leafNotAfter,
	}
	leaf := mkCert(t, leafTmpl, inter, &leafKey.PublicKey, interKey)

	return &testJWSBuilder{root: root, intermediate: inter, leaf: leaf, leafKey: leafKey}
}

func (b *testJWSBuilder) sign(t *testing.T, payloadJSON string) string {
	t.Helper()
	return b.signWithAlg(t, payloadJSON, "ES256")
}

func (b *testJWSBuilder) signWithAlg(t *testing.T, payloadJSON, alg string) string {
	t.Helper()
	hdr := jwsHeader{
		Alg: alg,
		X5c: []string{
			base64.StdEncoding.EncodeToString(b.leaf.Raw),
			base64.StdEncoding.EncodeToString(b.intermediate.Raw),
			base64.StdEncoding.EncodeToString(b.root.Raw),
		},
	}
	hdrJSON, err := json.Marshal(hdr)
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	encHeader := base64.RawURLEncoding.EncodeToString(hdrJSON)
	encPayload := base64.RawURLEncoding.EncodeToString([]byte(payloadJSON))
	signingInput := encHeader + "." + encPayload

	digest := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, b.leafKey, digest[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func (b *testJWSBuilder) signTampered(t *testing.T, payloadJSON string) string {
	t.Helper()
	jws := b.sign(t, payloadJSON)
	parts := splitDots(jws)
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	sig[0] ^= 0xFF
	return parts[0] + "." + parts[1] + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func splitDots(s string) []string {
	out := make([]string, 0, 3)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

func genKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return key
}

func caTemplate(t *testing.T, cn string, notBefore, notAfter time.Time) *x509.Certificate {
	t.Helper()
	return &x509.Certificate{
		SerialNumber:          serial(t),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
}

func mkCert(t *testing.T, tmpl, parent *x509.Certificate, pub *ecdsa.PublicKey, signer *ecdsa.PrivateKey) *x509.Certificate {
	t.Helper()
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, pub, signer)
	if err != nil {
		t.Fatalf("create certificate %q: %v", tmpl.Subject.CommonName, err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse certificate %q: %v", tmpl.Subject.CommonName, err)
	}
	return cert
}

func serial(t *testing.T) *big.Int {
	t.Helper()
	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("serial: %v", err)
	}
	return n
}
