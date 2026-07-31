// Package appstoreserverapi is a small, independently-testable client for
// Apple's App Store Server API — specifically the Get Notification History
// endpoint the appstorereconcile worker polls to detect ASSN webhook
// delivery gaps. It mirrors internal/apns in spirit: .p8-key JWT auth, no
// domain/Postgres knowledge, a real HTTP client hardened with timeouts and a
// bounded response body.
package appstoreserverapi

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

const (
	// productionBaseURL and sandboxBaseURL are Apple's two well-known App Store
	// Server API hosts, not configurable infrastructure.
	productionBaseURL = "https://api.storekit.itunes.apple.com"
	sandboxBaseURL    = "https://api.storekit-sandbox.itunes.apple.com"

	// historyPath is the Get Notification History endpoint.
	historyPath = "/inApps/v1/notifications/history"

	// requestTimeout bounds a single HTTP request/attempt.
	requestTimeout = 30 * time.Second
	// maxRespBytes bounds a single page's response body. A page of
	// JWS-signed notification history is a few dozen entries at most;
	// 10 MiB is a generous ceiling matching the platform's outbound-client
	// default.
	maxRespBytes = 10 << 20
	// maxHistoryPages defensively caps pagination so a malformed or
	// adversarial hasMore:true response can never loop forever.
	maxHistoryPages = 1000
	// maxAttempts bounds the retries a single page fetch makes against
	// repeated 429 responses before giving up.
	maxAttempts = 5
	// defaultRetryAfter is used when a 429 response carries no parseable
	// Retry-After header.
	defaultRetryAfter = 5 * time.Second
)

// Options configures the App Store Server API client.
type Options struct {
	// SigningKey is the PEM contents of the .p8 auth key (PKCS8 EC private key).
	SigningKey string
	// KeyID is the 10-character key id carried in the JWT header (kid).
	KeyID string
	// IssuerID is the UUID issuer id carried in the JWT payload (iss).
	IssuerID string
	// BundleID is the App Store bundle id carried in the JWT payload (bid).
	// Callers reuse the existing AppleBundleID config — there is no separate
	// bundle-id config for this client.
	BundleID string
	// Environment selects the base URL: "Sandbox" or "Production".
	Environment string
}

// baseURL resolves the App Store Server API host for the configured
// environment. Anything other than a case-insensitive "sandbox" resolves to
// Production, matching the fail-safe default used elsewhere in this repo
// (an unrecognized/unset environment must never silently target Sandbox).
func (o Options) baseURL() string {
	if strings.EqualFold(o.Environment, "Sandbox") {
		return sandboxBaseURL
	}
	return productionBaseURL
}

func (o Options) validate() error {
	if o.KeyID == "" {
		return fmt.Errorf("%w: KeyID is empty", ErrInvalidOptions)
	}
	if o.IssuerID == "" {
		return fmt.Errorf("%w: IssuerID is empty", ErrInvalidOptions)
	}
	if o.BundleID == "" {
		return fmt.Errorf("%w: BundleID is empty", ErrInvalidOptions)
	}
	return nil
}

// NotificationHistoryItem is one entry from the Get Notification History
// response: the still-signed (not yet verified) outer notification JWS.
type NotificationHistoryItem struct {
	SignedPayload string
}

// Client is a direct App Store Server API client, scoped to Get Notification
// History. It mints one ES256 JWT per FetchNotificationHistory call and
// reuses it across every paginated request that call makes.
type Client struct {
	http     *http.Client
	key      *ecdsa.PrivateKey
	keyID    string
	issuerID string
	bundleID string
	baseURL  string
	now      func() time.Time
}

// NewClient builds a client from validated options. now supplies the clock
// the minted JWT's iat/exp and Retry-After computations use (production
// passes time.Now; tests pin it).
func NewClient(opts Options, now func() time.Time) (*Client, error) {
	return newClientWithBaseURL(opts, opts.baseURL(), &http.Client{Timeout: requestTimeout}, now)
}

// newClientWithBaseURL is the constructor seam shared by NewClient and tests:
// an explicit base URL and HTTP client let an httptest server exercise the
// request/response logic.
func newClientWithBaseURL(opts Options, baseURL string, httpClient *http.Client, now func() time.Time) (*Client, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}
	key, err := parseP8Key([]byte(opts.SigningKey))
	if err != nil {
		return nil, err
	}
	return &Client{
		http:     httpClient,
		key:      key,
		keyID:    opts.KeyID,
		issuerID: opts.IssuerID,
		bundleID: opts.BundleID,
		baseURL:  baseURL,
		now:      now,
	}, nil
}

// historyRequest is the Get Notification History POST body. Only startDate
// and endDate are ever set — no optional filter fields (notificationType,
// notificationSubtype, transactionId, onlyFailures): gap detection is driven
// by our own idempotency table, not Apple's bookkeeping, so the full
// unfiltered window is pulled.
type historyRequest struct {
	StartDate       int64  `json:"startDate"`
	EndDate         int64  `json:"endDate"`
	PaginationToken string `json:"paginationToken,omitempty"`
}

type historyResponseItem struct {
	SignedPayload string `json:"signedPayload"`
}

type historyResponse struct {
	NotificationHistory []historyResponseItem `json:"notificationHistory"`
	HasMore             bool                  `json:"hasMore"`
	PaginationToken     string                `json:"paginationToken"`
}

// FetchNotificationHistory pages through Get Notification History for
// [start, end), following paginationToken/hasMore until exhausted (capped at
// maxHistoryPages), and returns every item across all pages. The JWT is
// minted once at the start of the call and reused for every page.
func (c *Client) FetchNotificationHistory(ctx context.Context, start, end time.Time) ([]NotificationHistoryItem, error) {
	token, err := mintJWT(c.key, c.keyID, c.issuerID, c.bundleID, c.now())
	if err != nil {
		return nil, fmt.Errorf("appstoreserverapi: mint jwt: %w", err)
	}

	var items []NotificationHistoryItem
	paginationToken := ""
	for page := 0; ; page++ {
		if page >= maxHistoryPages {
			return nil, fmt.Errorf("%w: exceeded %d pages", ErrTooManyPages, maxHistoryPages)
		}

		resp, err := c.fetchPage(ctx, token, start, end, paginationToken)
		if err != nil {
			return nil, err
		}
		for _, it := range resp.NotificationHistory {
			items = append(items, NotificationHistoryItem(it))
		}
		if !resp.HasMore {
			return items, nil
		}
		paginationToken = resp.PaginationToken
	}
}

// fetchPage posts one Get Notification History page, retrying a bounded
// number of times on 429 (honoring Retry-After in both the seconds and
// HTTP-date forms). Any other non-2xx status fails immediately with no
// retry.
func (c *Client) fetchPage(ctx context.Context, jwtToken string, start, end time.Time, paginationToken string) (historyResponse, error) {
	body, err := json.Marshal(historyRequest{
		StartDate:       start.UnixMilli(),
		EndDate:         end.UnixMilli(),
		PaginationToken: paginationToken,
	})
	if err != nil {
		return historyResponse{}, fmt.Errorf("appstoreserverapi: marshal request body: %w", err)
	}

	for attempt := 1; ; attempt++ {
		status, retryAfter, respBody, err := c.doOnce(ctx, jwtToken, body)
		if err != nil {
			return historyResponse{}, err
		}

		if status == http.StatusTooManyRequests {
			if attempt >= maxAttempts {
				return historyResponse{}, fmt.Errorf("%w: gave up after %d attempts", ErrRateLimited, attempt)
			}
			wait, ok := parseRetryAfter(retryAfter, c.now())
			if !ok {
				wait = defaultRetryAfter
			}
			if err := platform.Sleep(ctx, wait); err != nil {
				return historyResponse{}, err
			}
			continue
		}

		if status < 200 || status >= 300 {
			return historyResponse{}, fmt.Errorf("%w: %d", ErrUnexpectedStatus, status)
		}

		var parsed historyResponse
		if err := json.Unmarshal(respBody, &parsed); err != nil {
			return historyResponse{}, fmt.Errorf("appstoreserverapi: decode response: %w", err)
		}
		return parsed, nil
	}
}

// doOnce performs a single HTTP attempt and returns the status code, the raw
// Retry-After header value (empty when absent), and the response body
// (bounded by maxRespBytes).
func (c *Client) doOnce(ctx context.Context, jwtToken string, body []byte) (status int, retryAfter string, respBody []byte, err error) {
	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.baseURL+historyPath, bytes.NewReader(body))
	if err != nil {
		return 0, "", nil, fmt.Errorf("appstoreserverapi: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, "", nil, fmt.Errorf("appstoreserverapi: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err = io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
	if err != nil {
		return 0, "", nil, fmt.Errorf("appstoreserverapi: read response: %w", err)
	}

	return resp.StatusCode, resp.Header.Get("Retry-After"), respBody, nil
}
