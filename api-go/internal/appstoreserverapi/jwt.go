package appstoreserverapi

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
	"time"
)

// jwtValidity is how long a minted JWT is valid for, measured from iat. Apple
// requires exp <= iat+3600 (60 minutes); this reconciliation job mints exactly
// one token per short-lived job execution (no caching/refresh, unlike the
// long-lived APNs client), so a comfortable margin under Apple's ceiling is
// used rather than the full hour.
const jwtValidity = 55 * time.Minute

// jwtHeader is the JWS header the App Store Server API expects: ES256 with
// the 10-character key id and an explicit typ.
type jwtHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	Typ string `json:"typ"`
}

// jwtClaims is the JWS payload App Store Connect API auth requires: issuer,
// issued-at, expiry, audience, and the bundle id.
type jwtClaims struct {
	Iss string `json:"iss"`
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
	Aud string `json:"aud"`
	Bid string `json:"bid"`
}

// parseP8Key decodes a PEM block and parses the PKCS8-encoded EC private key
// Apple ships as a .p8 file — the same shape internal/apns/jwt.go parses.
func parseP8Key(pemBytes []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("%w", ErrInvalidSigningKey)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse pkcs8 signing key: %w", err)
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("%w: signing key is not an ECDSA key", ErrInvalidSigningKey)
	}
	return key, nil
}

// mintJWT signs a fresh ES256 JWT for one App Store Server API call. Minted
// once per job execution — see jwtValidity — never cached or refreshed.
func mintJWT(key *ecdsa.PrivateKey, keyID, issuerID, bundleID string, now time.Time) (string, error) {
	headerJSON, err := json.Marshal(jwtHeader{Alg: "ES256", Kid: keyID, Typ: "JWT"})
	if err != nil {
		return "", fmt.Errorf("marshal jwt header: %w", err)
	}

	iat := now.Unix()
	claimsJSON, err := json.Marshal(jwtClaims{
		Iss: issuerID,
		Iat: iat,
		Exp: iat + int64(jwtValidity.Seconds()),
		Aud: "appstoreconnect-v1",
		Bid: bundleID,
	})
	if err != nil {
		return "", fmt.Errorf("marshal jwt claims: %w", err)
	}

	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(claimsJSON)

	digest := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, key, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
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
