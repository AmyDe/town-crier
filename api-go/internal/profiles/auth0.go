package profiles

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// Auth0Manager is the consumer-side interface the profile handlers use to keep
// Auth0's app_metadata in sync and to remove users on account deletion. Both the
// real *Auth0Client and the NoOpAuth0Client satisfy it; the handlers never see
// the concrete type.
type Auth0Manager interface {
	UpdateSubscriptionTier(ctx context.Context, userID, tier string) error
	DeleteUser(ctx context.Context, userID string) error
}

// tokenExpirySkew shortens the cached token's lifetime by 60 seconds so it is
// refreshed before Auth0 considers it expired.
const tokenExpirySkew = 60 * time.Second

// auth0RequestTimeout bounds every outbound call to the Auth0 Management API.
const auth0RequestTimeout = 15 * time.Second

// maxAuth0ResponseBytes caps the response body read from Auth0.
const maxAuth0ResponseBytes = 1 << 20

// Auth0Client is a hand-rolled Auth0 Management API client using the M2M
// client-credentials grant with an in-memory cached token. Supports PATCH
// app_metadata.subscription_tier and DELETE user (tolerating 404). baseURL is
// "https://{domain}" in production; tests point it at an httptest server.
type Auth0Client struct {
	httpClient   *http.Client
	baseURL      string
	clientID     string
	clientSecret platform.SecretString

	mu          sync.Mutex
	cachedToken string
	tokenExpiry time.Time
}

// NewAuth0Client builds a client against baseURL (no trailing slash). The shared
// http.Client should carry a timeout; each request also bounds its own context.
func NewAuth0Client(httpClient *http.Client, baseURL, clientID string, clientSecret platform.SecretString) *Auth0Client {
	return &Auth0Client{
		httpClient:   httpClient,
		baseURL:      baseURL,
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

// UpdateSubscriptionTier PATCHes the user's app_metadata.subscription_tier.
func (c *Auth0Client) UpdateSubscriptionTier(ctx context.Context, userID, tier string) error {
	token, err := c.token(ctx)
	if err != nil {
		return err
	}

	body, err := json.Marshal(map[string]any{
		"app_metadata": map[string]any{"subscription_tier": tier},
	})
	if err != nil {
		return fmt.Errorf("marshal app_metadata: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, auth0RequestTimeout)
	defer cancel()
	// gosec G704: the host comes from the trusted AUTH0_DOMAIN config and userID
	// is the authenticated caller's own escaped subject — no attacker-controlled
	// host can be reached, so this is not an SSRF vector.
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPatch, c.userURL(userID), bytes.NewReader(body)) //nolint:gosec // trusted host + escaped own subject
	if err != nil {
		return fmt.Errorf("build patch request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req) //nolint:gosec // host from trusted config, not user input
	if err != nil {
		return fmt.Errorf("patch user %q: %w", userID, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("patch user %q: unexpected status %d", userID, resp.StatusCode)
	}
	return nil
}

// DeleteUser DELETEs the Auth0 user, tolerating 404 (the user is already gone,
// which is the desired end state).
func (c *Auth0Client) DeleteUser(ctx context.Context, userID string) error {
	token, err := c.token(ctx)
	if err != nil {
		return err
	}

	reqCtx, cancel := context.WithTimeout(ctx, auth0RequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodDelete, c.userURL(userID), nil) //nolint:gosec // trusted host + escaped own subject
	if err != nil {
		return fmt.Errorf("build delete request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req) //nolint:gosec // host from trusted config, not user input
	if err != nil {
		return fmt.Errorf("delete user %q: %w", userID, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("delete user %q: unexpected status %d", userID, resp.StatusCode)
	}
	return nil
}

// token returns a valid management-API access token, minting a fresh one via the
// client-credentials grant only when the cached token is absent or expired.
func (c *Auth0Client) token(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cachedToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.cachedToken, nil
	}

	body, err := json.Marshal(map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     c.clientID,
		"client_secret": c.clientSecret.Expose(),
		"audience":      c.baseURL + "/api/v2/",
	})
	if err != nil {
		return "", fmt.Errorf("marshal token request: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, auth0RequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.baseURL+"/oauth/token", bytes.NewReader(body)) //nolint:gosec // host from trusted AUTH0_DOMAIN config
	if err != nil {
		return "", fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req) //nolint:gosec // host from trusted config, not user input
	if err != nil {
		return "", fmt.Errorf("request token: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("request token: unexpected status %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxAuth0ResponseBytes))
	if err != nil {
		return "", fmt.Errorf("read token response: %w", err)
	}
	if err := json.Unmarshal(raw, &tokenResp); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("token response missing access_token")
	}

	c.cachedToken = tokenResp.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn)*time.Second - tokenExpirySkew)
	return c.cachedToken, nil
}

// userURL builds the management-API user resource URL with the user id escaped.
// url.PathEscape leaves sub-delims like "|" unescaped, but Auth0 ids carry "|"
// (e.g. "auth0|abc"); escapeUserID encodes "|" and other sub-delims so the
// resource path is correctly percent-encoded.
func (c *Auth0Client) userURL(userID string) string {
	return c.baseURL + "/api/v2/users/" + escapeUserID(userID)
}

// escapeUserID escapes a user id for use as a single path segment, encoding
// "|" and other sub-delims. url.QueryEscape matches for the realistic id forms
// (provider|id, email) which never contain spaces; the only divergence —
// space -> "+" — cannot occur in an Auth0 subject.
func escapeUserID(userID string) string {
	return url.QueryEscape(userID)
}

// NoOpAuth0Client is the fallback used when the Auth0 M2M config is absent:
// every operation succeeds without contacting Auth0.
type NoOpAuth0Client struct{}

// UpdateSubscriptionTier does nothing and succeeds.
func (NoOpAuth0Client) UpdateSubscriptionTier(_ context.Context, _, _ string) error { return nil }

// DeleteUser does nothing and succeeds.
func (NoOpAuth0Client) DeleteUser(_ context.Context, _ string) error { return nil }
