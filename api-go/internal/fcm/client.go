package fcm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

const (
	// maxRespBytes bounds the FCM error body read. FCM error bodies are a few
	// hundred bytes; 64 KiB is a generous ceiling.
	maxRespBytes = 64 << 10
	// requestTimeout bounds a single FCM (or token) HTTP request.
	requestTimeout = 30 * time.Second
)

// fcmErrorResponse is the FCM v1 non-2xx body. The google.rpc.Status carries the
// canonical status (e.g. NOT_FOUND); the FcmError detail carries the
// FCM-specific errorCode (e.g. UNREGISTERED) used to decide whether a token is
// permanently invalid.
type fcmErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Status  string `json:"status"`
		Message string `json:"message"`
		Details []struct {
			ErrorCode string `json:"errorCode"`
		} `json:"details"`
	} `json:"error"`
}

// Client is a direct FCM HTTP v1 push sender. It posts one request per device
// token (v1 has no multicast), attaches a cached service-account OAuth bearer,
// injects each token into the message body, and reports tokens FCM has rejected
// as permanently invalid (UNREGISTERED / INVALID_ARGUMENT) so the caller can
// prune them. It mirrors apns.Client's shape and per-device-failure contract.
type Client struct {
	http      *http.Client
	tokens    *tokenProvider
	projectID string
	baseURL   string
	logger    *slog.Logger
	now       func() time.Time
}

// NewClient builds a production FCM client from validated options. The caller
// guarantees opts.Enabled is true; when disabled, wire a NoOpSender instead.
func NewClient(opts Options, logger *slog.Logger, now func() time.Time) (*Client, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}

	httpClient := &http.Client{Timeout: requestTimeout}
	// Wrap the transport so every FCM push emits an OTel client span (Type=HTTP
	// in AppDependencies) named "FCM push". The static span name keeps
	// cardinality low (no per-device token in the name), matching the APNs client.
	httpClient = platform.WrapHTTPClient(httpClient, func(string, *http.Request) string { return "FCM push" })
	return newClientWithBaseURL(opts, productionBaseURL, httpClient, logger, now)
}

// newClientWithBaseURL is the constructor seam shared by NewClient and tests. It
// takes an explicit FCM base URL and HTTP client; the token endpoint URL comes
// from the service-account JSON, so a test can point both at httptest servers.
func newClientWithBaseURL(opts Options, baseURL string, httpClient *http.Client, logger *slog.Logger, now func() time.Time) (*Client, error) {
	tokens, err := newTokenProvider([]byte(opts.ServiceAccountJSON), httpClient, now)
	if err != nil {
		return nil, err
	}
	return &Client{
		http:      httpClient,
		tokens:    tokens,
		projectID: opts.ProjectID,
		baseURL:   baseURL,
		logger:    logger,
		now:       now,
	}, nil
}

// Send posts payload to each device token and returns the subset of tokens FCM
// rejected as permanently invalid. A per-device transport or server error is
// logged and skipped (the token is left for the next cycle), never returned as
// an error — one bad device must not abort the rest. payload is the FCM v1
// "message" object WITHOUT the token; Send injects each token itself.
func (c *Client) Send(ctx context.Context, tokens []string, payload json.RawMessage) ([]string, error) {
	if len(tokens) == 0 {
		return nil, nil
	}

	invalid := make([]string, 0, len(tokens))
	for _, token := range tokens {
		rejected, err := c.sendOne(ctx, token, payload)
		if err != nil {
			c.logger.ErrorContext(ctx, "fcm send failed", "token", redactToken(token), "error", err)
			continue
		}
		if rejected {
			invalid = append(invalid, token)
		}
	}
	if len(invalid) == 0 {
		return nil, nil
	}
	return invalid, nil
}

// sendOne posts payload to a single device token, returning rejected=true when
// FCM reports the token is permanently invalid. There is no retry loop day-1
// (parity with the epic's APNs-parity scope): a transient 5xx/429 is logged and
// skipped, the token left for the next cycle.
func (c *Client) sendOne(ctx context.Context, token string, payload json.RawMessage) (rejected bool, err error) {
	bearer, err := c.tokens.current(ctx)
	if err != nil {
		return false, fmt.Errorf("fcm: obtain access token: %w", err)
	}

	body, err := withToken(payload, token)
	if err != nil {
		return false, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	url := c.baseURL + "/v1/projects/" + c.projectID + "/messages:send"
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("fcm: build request: %w", err)
	}
	req.Header.Set("authorization", "Bearer "+bearer)
	req.Header.Set("content-type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return false, fmt.Errorf("fcm: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
	if err != nil {
		return false, fmt.Errorf("fcm: read response: %w", err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return false, nil
	}

	status, errorCode := parseError(respBody)
	if isInvalidToken(resp.StatusCode, status, errorCode) {
		c.logger.InfoContext(ctx, "fcm token rejected", "token", redactToken(token), "status", status, "errorCode", errorCode)
		return true, nil
	}
	c.logger.WarnContext(ctx, "fcm unhandled status", "http", resp.StatusCode, "status", status, "errorCode", errorCode, "token", redactToken(token))
	return false, nil
}

// isInvalidToken reports whether an FCM error identifies the device token itself
// as permanently invalid (so it should be pruned), as opposed to a transient or
// server-side fault. UNREGISTERED (device uninstalled / token rotated) and
// INVALID_ARGUMENT (malformed token) are terminal; a NOT_FOUND canonical status
// (HTTP 404) covers the same "token gone" case when no FcmError detail is
// present.
func isInvalidToken(httpStatus int, status, errorCode string) bool {
	switch errorCode {
	case "UNREGISTERED", "INVALID_ARGUMENT":
		return true
	}
	if status == "NOT_FOUND" || httpStatus == http.StatusNotFound {
		return true
	}
	return false
}

// parseError extracts the canonical status and the FCM-specific errorCode from a
// non-2xx FCM body, returning empty strings when the body is absent or not the
// expected shape.
func parseError(body []byte) (status, errorCode string) {
	if len(body) == 0 {
		return "", ""
	}
	var parsed fcmErrorResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", ""
	}
	if len(parsed.Error.Details) > 0 {
		errorCode = parsed.Error.Details[0].ErrorCode
	}
	return parsed.Error.Status, errorCode
}

// withToken injects the per-recipient token into a token-less FCM message object
// and wraps it in the {"message":{...}} envelope the v1 send endpoint expects.
// The message fields are preserved verbatim; only the token key is added.
func withToken(message json.RawMessage, token string) ([]byte, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(message, &fields); err != nil {
		return nil, fmt.Errorf("fcm: parse message payload: %w", err)
	}
	tokenJSON, err := json.Marshal(token)
	if err != nil {
		return nil, fmt.Errorf("fcm: marshal token: %w", err)
	}
	fields["token"] = tokenJSON

	envelope := struct {
		Message map[string]json.RawMessage `json:"message"`
	}{Message: fields}
	return json.Marshal(envelope)
}

// redactToken keeps only an 8-character prefix so device tokens — which identify
// a user's device — never land in logs in full.
func redactToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:8] + "..."
}

// compile-time assertions that both senders satisfy the consumer contract.
var (
	_ PushSender = (*Client)(nil)
	_ PushSender = (*NoOpSender)(nil)
)
