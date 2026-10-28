package apns

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// devicePathPrefix is the APNs URL path segment do() places the raw device
// token after; redactPushURL uses it to find and redact the token before the
// URL is recorded into the "APNs push" client span's url.full attribute.
const devicePathPrefix = "/3/device/"

// tracerName labels the "APNs delivery" wrapper span this package emits,
// distinct from the transport-level "APNs push" HTTP client spans
// platform.WrapHTTPClient attaches to each individual request/retry.
const tracerName = "github.com/AmyDe/town-crier/api-go/internal/apns"

const (
	// maxAttempts bounds retries per device on transient (5xx / transport)
	// failures. The expired-provider-token retry is counted separately.
	maxAttempts = 3
	// initialBackoff is the first retry delay; it doubles each attempt.
	initialBackoff = 100 * time.Millisecond
	// maxRespBytes bounds the APNs error body read. Apple's error bodies are a
	// few dozen bytes; 64 KiB is a generous ceiling.
	maxRespBytes = 64 << 10
	// requestTimeout bounds a single APNs HTTP request.
	requestTimeout = 30 * time.Second
)

// errorResponse is the body APNs returns on a non-2xx response, e.g.
// {"reason":"BadDeviceToken"}.
type errorResponse struct {
	Reason string `json:"reason"`
}

// Client is a direct APNs HTTP/2 push sender. It posts one request per device
// token, attaches a cached ES256-signed provider JWT, and reports tokens APNs
// has rejected as permanently invalid (410 Unregistered, 400 BadDeviceToken) so
// the caller can prune them.
type Client struct {
	http     *http.Client
	jwt      *jwtProvider
	baseURL  string
	bundleID string
	logger   *slog.Logger
	metrics  pushMetricsRecorder
}

// pushMetricsRecorder is the consumer-side slice of the metrics registry the
// client records towncrier.push.delivery_failed on. *metrics.Registry
// satisfies it; nil (the default) leaves the counter dark, matching the
// notifydispatch.Enqueuer.WithMetrics convention.
type pushMetricsRecorder interface {
	PushDeliveryFailed(ctx context.Context, platform string)
}

// WithMetrics wires the recorder Send calls PushDeliveryFailed on for a
// genuine per-device delivery failure (never for a routine invalid-token
// rejection). Returns c for chaining; the default nil recorder leaves the
// counter dark.
func (c *Client) WithMetrics(rec pushMetricsRecorder) *Client {
	c.metrics = rec
	return c
}

// NewClient builds a production APNs client from validated options. The HTTP
// client uses an HTTP/2-only transport — APNs requires h2 — with TLS 1.2 as the
// floor. The caller guarantees opts.Enabled is true; when disabled, wire a
// NoOpSender instead.
func NewClient(opts Options, logger *slog.Logger, now func() time.Time) (*Client, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}

	httpClient := tracedPushClient(newTransport())
	return newClientWithBaseURL(opts, opts.baseURL(), httpClient, logger, now)
}

// newTransport returns an HTTP/2-only transport (APNs requires h2) with a TLS
// 1.2 floor. The ping timeouts make a silently dead connection fail fast so
// the next send redials.
func newTransport() *http.Transport {
	var protocols http.Protocols
	protocols.SetHTTP2(true)
	return &http.Transport{
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		Protocols:       &protocols,
		HTTP2: &http.HTTP2Config{
			SendPingTimeout: 30 * time.Second,
			PingTimeout:     15 * time.Second,
		},
	}
}

// tracedPushClient applies the redaction-then-tracing chain NewClient uses, so
// tests exercise the real construction seam against a stub transport.
func tracedPushClient(base http.RoundTripper, opts ...otelhttp.Option) *http.Client {
	httpClient := &http.Client{
		Timeout: requestTimeout,
	}
	// Redaction must wrap the transport, not the client, so otelhttp starts the
	// span first and url.full is overwritten before the request is sent.
	httpClient.Transport = &platform.RedactingTransport{Next: base, Redact: redactPushURL}
	// The static span name keeps cardinality low (no device token).
	return platform.WrapHTTPClient(httpClient, func(string, *http.Request) string { return "APNs push" }, opts...)
}

// newClientWithBaseURL is the constructor seam shared by NewClient and tests. It
// takes an explicit base URL and HTTP client so an httptest server (HTTP/1.1)
// can exercise the request/response logic without a real h2 endpoint.
func newClientWithBaseURL(opts Options, baseURL string, httpClient *http.Client, logger *slog.Logger, now func() time.Time) (*Client, error) {
	jwt, err := newJWTProvider([]byte(opts.AuthKey), opts.KeyID, opts.TeamID, now)
	if err != nil {
		return nil, err
	}
	return &Client{
		http:     httpClient,
		jwt:      jwt,
		baseURL:  baseURL,
		bundleID: opts.BundleID,
		logger:   logger,
	}, nil
}

// Send posts payload to each device token and returns the subset of tokens APNs
// rejected as permanently invalid. A per-device transport or server error is
// logged and skipped (the token is left for the next cycle), never returned as
// an error — one bad device must not abort the rest. The payload is the
// caller-built APNs JSON body (the {"aps":...} document).
func (c *Client) Send(ctx context.Context, tokens []string, payload json.RawMessage) ([]string, error) {
	if len(tokens) == 0 {
		return nil, nil
	}

	invalid := make([]string, 0, len(tokens))
	for _, token := range tokens {
		rejected, err := c.sendOneTraced(ctx, token, payload)
		if err != nil {
			c.logger.ErrorContext(ctx, "apns send failed", "token", redactToken(token), "error", err)
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

// sendOneTraced wraps sendOne in an "APNs delivery" span and records
// towncrier.push.delivery_failed on failure. A permanently-invalid-token
// rejection (rejected=true, err=nil) is expected churn and is not a failure.
func (c *Client) sendOneTraced(ctx context.Context, token string, payload json.RawMessage) (rejected bool, err error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "APNs delivery")
	defer span.End()

	rejected, err = c.sendOne(ctx, token, payload)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if c.metrics != nil {
			c.metrics.PushDeliveryFailed(ctx, "apns")
		}
	}
	return rejected, err
}

// sendOne posts payload to a single device token, returning rejected=true when
// APNs reports the token is permanently invalid. It refreshes the JWT once on a
// 403 ExpiredProviderToken and retries idempotent 5xx / transport failures with
// exponential backoff.
func (c *Client) sendOne(ctx context.Context, token string, payload json.RawMessage) (rejected bool, err error) {
	backoff := initialBackoff
	jwtRefreshed := false

	for attempt := 1; ; attempt++ {
		resp, sendErr := c.do(ctx, token, payload)
		if sendErr != nil {
			if attempt < maxAttempts {
				if waitErr := platform.Sleep(ctx, backoff); waitErr != nil {
					return false, waitErr
				}
				backoff *= 2
				continue
			}
			return false, sendErr
		}

		status := resp.status
		reason := resp.reason

		switch {
		case status >= 200 && status < 300:
			return false, nil
		case status == http.StatusGone:
			c.logger.InfoContext(ctx, "apns token unregistered", "token", redactToken(token))
			return true, nil
		case status == http.StatusBadRequest && reason == "BadDeviceToken":
			c.logger.WarnContext(ctx, "apns bad device token", "token", redactToken(token))
			return true, nil
		case status == http.StatusForbidden && reason == "ExpiredProviderToken" && !jwtRefreshed:
			c.logger.InfoContext(ctx, "apns expired provider token; refreshing jwt")
			c.jwt.invalidate()
			jwtRefreshed = true
			continue
		case status == http.StatusTooManyRequests && reason == "TooManyProviderTokenUpdates":
			c.logger.WarnContext(ctx, "apns too many provider token updates; deferring")
			return false, nil
		case status >= 500 && status < 600 && attempt < maxAttempts:
			c.logger.WarnContext(ctx, "apns transient error", "status", status, "reason", reason, "attempt", attempt)
			if waitErr := platform.Sleep(ctx, backoff); waitErr != nil {
				return false, waitErr
			}
			backoff *= 2
			continue
		default:
			c.logger.WarnContext(ctx, "apns unhandled status", "status", status, "reason", reason, "token", redactToken(token))
			return false, nil
		}
	}
}

// apnsResponse carries the parsed status and reason from one APNs request.
type apnsResponse struct {
	status int
	reason string
}

// do performs a single APNs request, reading and closing the response body.
func (c *Client) do(ctx context.Context, token string, payload json.RawMessage) (apnsResponse, error) {
	jwtToken, err := c.jwt.current()
	if err != nil {
		return apnsResponse{}, fmt.Errorf("apns: mint jwt: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	url := c.baseURL + devicePathPrefix + token
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return apnsResponse{}, fmt.Errorf("apns: build request: %w", err)
	}
	req.Header.Set("authorization", "bearer "+jwtToken)
	req.Header.Set("apns-topic", c.bundleID)
	req.Header.Set("apns-push-type", "alert")
	req.Header.Set("apns-priority", "10")
	req.Header.Set("content-type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return apnsResponse{}, fmt.Errorf("apns: request: %w", redactTransportError(err, token))
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
	if err != nil {
		return apnsResponse{}, fmt.Errorf("apns: read response: %w", err)
	}

	return apnsResponse{status: resp.StatusCode, reason: parseReason(body)}, nil
}

// parseReason extracts the APNs error reason from a response body, returning the
// empty string when the body is absent or not the expected shape.
func parseReason(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var parsed errorResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ""
	}
	return parsed.Reason
}

// redactTransportError rebuilds the *url.Error that http.Client.Do returns so
// its URL carries the redacted device token. The inner error is kept, so
// errors.Is and Timeout() classification still work; any remaining raw token in
// the text of a non-URL error is masked by wrapping it.
func redactTransportError(err error, token string) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = &url.Error{
			Op:  urlErr.Op,
			URL: strings.ReplaceAll(urlErr.URL, token, redactToken(token)),
			Err: urlErr.Err,
		}
	}
	if token != "" && strings.Contains(err.Error(), token) {
		return maskedError{err: err, msg: strings.ReplaceAll(err.Error(), token, redactToken(token))}
	}
	return err
}

// maskedError reports msg as its text while keeping err in the unwrap chain.
type maskedError struct {
	err error
	msg string
}

func (m maskedError) Error() string { return m.msg }
func (m maskedError) Unwrap() error { return m.err }

// redactToken keeps only an 8-character prefix so device tokens — which identify
// a user's device — never land in logs in full.
func redactToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:8] + "..."
}

// redactPushURL rewrites an APNs request URL for telemetry so the device
// token in the /3/device/<token> path (see do()) is reduced to the same
// 8-char-prefix redaction the logging layer already uses, instead of shipping
// the raw token into the "APNs push" client span's url.full attribute
// (App Insights, 90d retention). Wired as a platform.URLRedactor via
// platform.RedactingTransport in NewClient. If the expected path prefix isn't
// present — defensive, since do() always builds the URL this way — the URL is
// returned unchanged rather than guessing at a redaction.
func redactPushURL(r *http.Request) string {
	full := r.URL.String()
	idx := strings.Index(full, devicePathPrefix)
	if idx < 0 {
		return full
	}
	tokenStart := idx + len(devicePathPrefix)
	return full[:tokenStart] + redactToken(full[tokenStart:])
}

// compile-time assertions that both senders satisfy the consumer contract.
var (
	_ PushSender = (*Client)(nil)
	_ PushSender = (*NoOpSender)(nil)
)
