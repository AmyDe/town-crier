package apns

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"sync"
	"time"
)

// refreshInterval is how long a minted provider JWT is reused before a fresh
// one is signed. Apple rejects a token older than 60 minutes (403
// ExpiredProviderToken) and rejects re-mints more often than once per 20 minutes
// (429 TooManyProviderTokenUpdates). Refreshing at 50 minutes stays safely
// inside both bounds.
const refreshInterval = 50 * time.Minute

// jwtHeader is the JWS header Apple expects: ES256 with the 10-character key id.
type jwtHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
}

// jwtClaims is the JWS payload: the team id as issuer and the issued-at epoch.
type jwtClaims struct {
	Iss string `json:"iss"`
	Iat int64  `json:"iat"`
}

// jwtProvider mints and caches an ES256-signed APNs provider JWT. A single
// token is held in memory and re-signed under a mutex; the provider is intended
// to be shared by a single Client across all device requests.
type jwtProvider struct {
	key   *ecdsa.PrivateKey
	keyID string
	teamID string
	now   func() time.Time

	mu       sync.Mutex
	cached   string
	mintedAt time.Time
}

// newJWTProvider parses a PKCS8 PEM .p8 auth key and returns a provider that
// signs JWTs with it. The keyID populates the JWS header (kid) and the teamID
// the payload (iss). now supplies the clock so tests can pin time.
func newJWTProvider(pemBytes []byte, keyID, teamID string, now func() time.Time) (*jwtProvider, error) {
	key, err := parseP8Key(pemBytes)
	if err != nil {
		return nil, err
	}
	return &jwtProvider{key: key, keyID: keyID, teamID: teamID, now: now}, nil
}

// parseP8Key decodes a PEM block and parses the PKCS8-encoded EC private key
// Apple ships as a .p8 file.
func parseP8Key(pemBytes []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("apns: %w", ErrInvalidAuthKey)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("apns: parse pkcs8 auth key: %w", err)
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("apns: %w: auth key is not an ECDSA key", ErrInvalidAuthKey)
	}
	return key, nil
}

// current returns the cached JWT, minting a fresh one when none exists or the
// cached token is older than the refresh interval.
func (p *jwtProvider) current() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := p.now()
	if p.cached != "" && now.Sub(p.mintedAt) <= refreshInterval {
		return p.cached, nil
	}

	token, err := p.mint(now)
	if err != nil {
		return "", err
	}
	p.cached = token
	p.mintedAt = now
	return token, nil
}

// invalidate drops the cached token so the next current call re-signs. The
// Client calls this on a 403 ExpiredProviderToken before retrying once.
func (p *jwtProvider) invalidate() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cached = ""
}

// mint signs a fresh JWS using ES256. crypto/ecdsa emits an ASN.1 DER
// signature; the JWS form is the fixed-width r||s concatenation, so the (r, s)
// pair is left-padded to the curve's 32-byte size.
func (p *jwtProvider) mint(now time.Time) (string, error) {
	headerJSON, err := json.Marshal(jwtHeader{Alg: "ES256", Kid: p.keyID})
	if err != nil {
		return "", fmt.Errorf("apns: marshal jwt header: %w", err)
	}
	claimsJSON, err := json.Marshal(jwtClaims{Iss: p.teamID, Iat: now.Unix()})
	if err != nil {
		return "", fmt.Errorf("apns: marshal jwt claims: %w", err)
	}

	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(claimsJSON)

	digest := sha256Sum([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, p.key, digest[:])
	if err != nil {
		return "", fmt.Errorf("apns: sign jwt: %w", err)
	}

	sig := jwsSignature(r, s)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// jwsSignature encodes an ECDSA (r, s) pair as the 64-byte r||s form ES256
// requires, left-padding each integer to 32 bytes.
func jwsSignature(r, s *big.Int) []byte {
	const size = 32
	sig := make([]byte, 2*size)
	r.FillBytes(sig[:size])
	s.FillBytes(sig[size:])
	return sig
}

// sha256Sum is a thin alias so the signing path and tests share one hash.
func sha256Sum(b []byte) [32]byte { return sha256.Sum256(b) }

// bigIntFromBytes reads a big-endian unsigned integer; used by tests verifying
// the JWS signature halves.
func bigIntFromBytes(b []byte) *big.Int { return new(big.Int).SetBytes(b) }
