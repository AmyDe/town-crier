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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// jwtBearerGrant is the OAuth2 grant type for the two-legged service-account
// flow: a signed JWT assertion is exchanged for an access token. It is a public,
// well-known grant-type URI, not a credential.
//
//nolint:gosec // G101: OAuth grant-type URI, not a hardcoded secret.
const jwtBearerGrant = "urn:ietf:params:oauth:grant-type:jwt-bearer"

// assertionTTL is how long a minted assertion JWT claims validity. Google caps
// service-account assertions at one hour; the assertion is short-lived and
// re-minted on every token fetch, so the full hour is fine.
const assertionTTL = time.Hour

// tokenExpiryMargin is subtracted from the access token's reported lifetime so a
// token is refreshed before it actually expires, absorbing clock skew and the
// send request's own latency.
const tokenExpiryMargin = 60 * time.Second

// maxTokenRespBytes bounds the OAuth token endpoint response read.
const maxTokenRespBytes = 64 << 10

// serviceAccount is the subset of a Firebase/GCP service-account key JSON the
// JWT-bearer flow needs. The full document carries more fields; only these are
// read.
type serviceAccount struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
	ProjectID   string `json:"project_id"`
}

// jwtHeader is the JWS header for the assertion: RS256, JWT type.
type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

// jwtClaims is the assertion payload: the service-account email as issuer, the
// FCM scope, the token endpoint as audience, and the issued-at / expiry epochs.
type jwtClaims struct {
	Iss   string `json:"iss"`
	Scope string `json:"scope"`
	Aud   string `json:"aud"`
	Iat   int64  `json:"iat"`
	Exp   int64  `json:"exp"`
}

// tokenResponse is the OAuth token endpoint's success body.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// tokenProvider mints an RS256-signed JWT assertion, exchanges it at the
// service-account token endpoint for an OAuth access token, and caches that
// token until shortly before it expires. It mirrors apns/jwt.go's cache-and-mint
// shape; FCM differs only in that obtaining a bearer requires a network round
// trip (the JWT is not itself the bearer), so current takes a context.
type tokenProvider struct {
	key         *rsa.PrivateKey
	clientEmail string
	tokenURI    string
	http        *http.Client
	now         func() time.Time

	mu        sync.Mutex
	cached    string
	expiresAt time.Time
}

// newTokenProvider parses a service-account JSON blob (its RSA private key,
// client email, and token URI) and returns a provider that exchanges signed
// assertions for access tokens over httpClient. now supplies the clock so tests
// can pin time.
func newTokenProvider(saJSON []byte, httpClient *http.Client, now func() time.Time) (*tokenProvider, error) {
	var sa serviceAccount
	if err := json.Unmarshal(saJSON, &sa); err != nil {
		return nil, fmt.Errorf("fcm: %w: parse json: %w", ErrInvalidServiceAccount, err)
	}
	if sa.ClientEmail == "" {
		return nil, fmt.Errorf("fcm: %w: client_email is empty", ErrInvalidServiceAccount)
	}
	if sa.TokenURI == "" {
		return nil, fmt.Errorf("fcm: %w: token_uri is empty", ErrInvalidServiceAccount)
	}
	key, err := parseRSAPrivateKey([]byte(sa.PrivateKey))
	if err != nil {
		return nil, err
	}
	return &tokenProvider{
		key:         key,
		clientEmail: sa.ClientEmail,
		tokenURI:    sa.TokenURI,
		http:        httpClient,
		now:         now,
	}, nil
}

// parseRSAPrivateKey decodes the PEM-wrapped PKCS8 RSA private key a Google
// service-account key carries in its private_key field.
func parseRSAPrivateKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("fcm: %w: private_key is not PEM", ErrInvalidServiceAccount)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("fcm: %w: parse pkcs8 private key: %w", ErrInvalidServiceAccount, err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("fcm: %w: private_key is not an RSA key", ErrInvalidServiceAccount)
	}
	return key, nil
}

// current returns a cached access token, fetching a fresh one when none is held
// or the cached token is within tokenExpiryMargin of expiry.
func (p *tokenProvider) current(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := p.now()
	if p.cached != "" && now.Before(p.expiresAt) {
		return p.cached, nil
	}

	token, lifetime, err := p.fetch(ctx, now)
	if err != nil {
		return "", err
	}
	p.cached = token
	p.expiresAt = now.Add(lifetime - tokenExpiryMargin)
	return token, nil
}

// fetch mints an assertion, posts it to the token endpoint, and returns the
// access token and its reported lifetime.
func (p *tokenProvider) fetch(ctx context.Context, now time.Time) (token string, lifetime time.Duration, err error) {
	assertion, err := p.mintAssertion(now)
	if err != nil {
		return "", 0, err
	}

	form := url.Values{
		"grant_type": {jwtBearerGrant},
		"assertion":  {assertion},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, fmt.Errorf("fcm: build token request: %w", err)
	}
	req.Header.Set("content-type", "application/x-www-form-urlencoded")

	resp, err := p.http.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("fcm: token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTokenRespBytes))
	if err != nil {
		return "", 0, fmt.Errorf("fcm: read token response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", 0, fmt.Errorf("fcm: token endpoint status %d: %s", resp.StatusCode, string(body))
	}

	var parsed tokenResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", 0, fmt.Errorf("fcm: parse token response: %w", err)
	}
	if parsed.AccessToken == "" {
		return "", 0, fmt.Errorf("fcm: token response carried no access_token")
	}
	return parsed.AccessToken, time.Duration(parsed.ExpiresIn) * time.Second, nil
}

// mintAssertion signs the RS256 JWT assertion the token endpoint exchanges for
// an access token.
func (p *tokenProvider) mintAssertion(now time.Time) (string, error) {
	headerJSON, err := json.Marshal(jwtHeader{Alg: "RS256", Typ: "JWT"})
	if err != nil {
		return "", fmt.Errorf("fcm: marshal jwt header: %w", err)
	}
	claimsJSON, err := json.Marshal(jwtClaims{
		Iss:   p.clientEmail,
		Scope: firebaseMessagingScope,
		Aud:   p.tokenURI,
		Iat:   now.Unix(),
		Exp:   now.Add(assertionTTL).Unix(),
	})
	if err != nil {
		return "", fmt.Errorf("fcm: marshal jwt claims: %w", err)
	}

	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(claimsJSON)

	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, p.key, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("fcm: sign jwt: %w", err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}
