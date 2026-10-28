package acsemail

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

const (
	// apiVersion pins the ACS Email REST contract version.
	apiVersion = "2023-03-31"
	// sendPath is the email send operation path appended to the account endpoint.
	sendPath = "/emails:send"
	// maxRespBytes bounds every ACS response body read.
	maxRespBytes = 1 << 20
	// requestTimeout bounds a single ACS HTTP request.
	requestTimeout = 30 * time.Second
	// defaultPollInterval is the delay between operation-status polls.
	defaultPollInterval = 2 * time.Second
	// defaultMaxPolls bounds how many times the send operation is polled before
	// giving up (≈ defaultMaxPolls × defaultPollInterval of wall time).
	defaultMaxPolls = 30
	// maxPollRetryAfter caps how long a throttled status poll waits, whatever
	// Retry-After asks for.
	maxPollRetryAfter = 30 * time.Second
)

// ErrSendFailed reports that ACS accepted the request but the email operation
// finished in a non-success terminal state, or never reached success within the
// poll budget.
var ErrSendFailed = errors.New("acsemail: send failed")

// ErrRateLimited reports that ACS rejected the send request with HTTP 429; the
// email was not accepted. Errors wrapping it also wrap ErrSendFailed.
var ErrRateLimited = errors.New("acsemail: rate limited")

// throttledError is a 429 on an operation-status poll, carrying how long to
// wait before polling again.
type throttledError struct {
	wait time.Duration
	body string
}

func (e *throttledError) Error() string {
	return "acsemail: poll returned status 429: " + e.body
}

// ErrAcceptedStatusUnknown reports that ACS accepted the send request (or
// reported the original request still in progress) but the final operation
// status could not be read. The email is probably delivered, so callers must
// not resend it.
var ErrAcceptedStatusUnknown = errors.New("acsemail: accepted, status unknown")

// Message is the email a caller asks the client to deliver. The HTML body and
// subject are pre-built by the caller.
type Message struct {
	// Sender is the verified ACS sender address (e.g. hello@towncrierapp.uk).
	Sender string
	// Recipient is the single destination address.
	Recipient string
	// Subject is the email subject line.
	Subject string
	// HTMLBody is the rendered HTML email body.
	HTMLBody string
	// RepeatabilityKey identifies the logical send. When set, the client derives
	// a deterministic Repeatability-Request-Id from it so ACS de-duplicates a
	// repeat of the same send. Empty disables repeatability headers.
	RepeatabilityKey string
}

// sendRequest is the ACS Email REST send payload shape.
type sendRequest struct {
	SenderAddress string         `json:"senderAddress"`
	Content       sendContent    `json:"content"`
	Recipients    sendRecipients `json:"recipients"`
}

type sendContent struct {
	Subject string `json:"subject"`
	HTML    string `json:"html"`
}

type sendRecipients struct {
	To []sendAddress `json:"to"`
}

type sendAddress struct {
	Address string `json:"address"`
}

// operationStatus is the polled operation-status body shape.
type operationStatus struct {
	ID     string          `json:"id"`
	Status string          `json:"status"`
	Error  *operationError `json:"error,omitempty"`
}

type operationError struct {
	Message string `json:"message"`
}

// Client sends transactional email through the Azure Communication Services
// Email REST API, signing each request with HMAC-SHA256 from the connection
// string's access key. It is built from stdlib only — there is no official Go
// ACS Email SDK.
type Client struct {
	creds        credentials
	http         *http.Client
	logger       *slog.Logger
	now          func() time.Time
	pollInterval time.Duration
	maxPolls     int
}

// NewClient parses the ACS connection string and returns a real sender. When
// the connection string is absent, wire a NoOpSender instead.
func NewClient(connectionString string, logger *slog.Logger, now func() time.Time) (*Client, error) {
	creds, err := parseConnectionString(connectionString)
	if err != nil {
		return nil, err
	}
	httpClient := &http.Client{Timeout: requestTimeout}
	// Wrap the transport so every ACS send/poll emits an OTel client span
	// (Type=HTTP in AppDependencies) named "ACS email send". The ACS host lands
	// in server.address; the static span name keeps cardinality low (no recipient
	// or operation ID in the name).
	httpClient = platform.WrapHTTPClient(httpClient, func(string, *http.Request) string { return "ACS email send" })
	return newClientWithCreds(creds, httpClient, logger, now), nil
}

// newClientWithCreds is the constructor seam shared by NewClient and tests.
func newClientWithCreds(creds credentials, httpClient *http.Client, logger *slog.Logger, now func() time.Time) *Client {
	return &Client{
		creds:        creds,
		http:         httpClient,
		logger:       logger,
		now:          now,
		pollInterval: defaultPollInterval,
		maxPolls:     defaultMaxPolls,
	}
}

// Send delivers one message: it POSTs the signed send request, then polls the
// returned Operation-Location until the operation reaches a terminal state.
// A rejected request or a failed terminal state returns an error wrapping
// ErrSendFailed. When ACS accepted the request but the poll errors or runs out
// of budget, the error wraps ErrAcceptedStatusUnknown.
func (c *Client) Send(ctx context.Context, msg Message) error {
	body, err := json.Marshal(sendRequest{
		SenderAddress: msg.Sender,
		Content:       sendContent{Subject: msg.Subject, HTML: msg.HTMLBody},
		Recipients:    sendRecipients{To: []sendAddress{{Address: msg.Recipient}}},
	})
	if err != nil {
		return fmt.Errorf("acsemail: marshal send request: %w", err)
	}

	var extra http.Header
	if msg.RepeatabilityKey != "" {
		extra = http.Header{}
		extra.Set("Repeatability-Request-Id", repeatabilityID(msg.RepeatabilityKey))
		extra.Set("Repeatability-First-Sent", c.now().UTC().Format(http.TimeFormat))
	}

	opLocation, err := c.postSend(ctx, body, extra)
	if err != nil {
		return err
	}

	if err := c.pollOperation(ctx, opLocation); err != nil {
		if errors.Is(err, ErrSendFailed) {
			return err
		}
		return fmt.Errorf("%w: %w", ErrAcceptedStatusUnknown, err)
	}
	return nil
}

// repeatabilityID derives a canonical lowercase GUID (version 8 layout) from
// key, so the same key always yields the same ID.
func repeatabilityID(key string) string {
	sum := sha256.Sum256([]byte(key))
	b := sum[:16]
	b[6] = b[6]&0x0f | 0x80
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// postSend signs and POSTs the email send request, returning the
// Operation-Location URL to poll for completion.
func (c *Client) postSend(ctx context.Context, body []byte, extra http.Header) (string, error) {
	endpoint := c.creds.endpoint + sendPath + "?api-version=" + apiVersion
	resp, err := c.signedRequest(ctx, http.MethodPost, endpoint, body, extra)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
	if err != nil {
		return "", fmt.Errorf("acsemail: read send response: %w", err)
	}
	if resp.StatusCode == http.StatusConflict {
		return "", fmt.Errorf("acsemail: %w: send returned status 409: %s", ErrAcceptedStatusUnknown, strings.TrimSpace(string(respBody)))
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return "", fmt.Errorf("acsemail: %w: %w: send returned status 429: %s", ErrSendFailed, ErrRateLimited, strings.TrimSpace(string(respBody)))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("acsemail: %w: send returned status %d: %s", ErrSendFailed, resp.StatusCode, string(respBody))
	}

	opLocation := resp.Header.Get("Operation-Location")
	if opLocation == "" {
		// Some accounts return the terminal status inline; treat an absent
		// Operation-Location plus a success status body as done.
		var status operationStatus
		if err := json.Unmarshal(respBody, &status); err == nil && isSuccess(status.Status) {
			return "", nil
		}
		return "", fmt.Errorf("acsemail: %w: send response had no Operation-Location", ErrSendFailed)
	}
	return opLocation, nil
}

// pollOperation polls the operation-status URL until the operation reaches a
// terminal state or the poll budget is exhausted. An empty location means the
// send already completed inline.
func (c *Client) pollOperation(ctx context.Context, opLocation string) error {
	if opLocation == "" {
		return nil
	}

	for range c.maxPolls {
		status, err := c.getOperationStatus(ctx, opLocation)
		if err != nil {
			var throttled *throttledError
			if errors.As(err, &throttled) {
				if err := platform.Sleep(ctx, throttled.wait); err != nil {
					return err
				}
				continue
			}
			return err
		}

		switch {
		case isSuccess(status.Status):
			c.logger.DebugContext(ctx, "acs email sent", "operation", status.ID)
			return nil
		case isTerminalFailure(status.Status):
			msg := ""
			if status.Error != nil {
				msg = status.Error.Message
			}
			return fmt.Errorf("acsemail: %w: operation %s status=%s: %s", ErrSendFailed, status.ID, status.Status, msg)
		}

		if err := platform.Sleep(ctx, c.pollInterval); err != nil {
			return err
		}
	}
	return fmt.Errorf("acsemail: operation did not complete within %d polls", c.maxPolls)
}

// getOperationStatus signs and GETs the operation-status URL.
func (c *Client) getOperationStatus(ctx context.Context, opLocation string) (operationStatus, error) {
	resp, err := c.signedRequest(ctx, http.MethodGet, opLocation, nil, nil)
	if err != nil {
		return operationStatus{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
	if err != nil {
		return operationStatus{}, fmt.Errorf("acsemail: read operation status: %w", err)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return operationStatus{}, &throttledError{
			wait: throttleWait(resp.Header.Get("Retry-After"), c.pollInterval),
			body: string(respBody),
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return operationStatus{}, fmt.Errorf("acsemail: poll returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var status operationStatus
	if err := json.Unmarshal(respBody, &status); err != nil {
		return operationStatus{}, fmt.Errorf("acsemail: decode operation status: %w", err)
	}
	return status, nil
}

// throttleWait turns a Retry-After header into a poll wait. Only delta-seconds
// is understood; an absent, malformed or negative value falls back to fallback.
// The result never exceeds maxPollRetryAfter.
func throttleWait(retryAfter string, fallback time.Duration) time.Duration {
	wait := fallback
	if secs, err := strconv.Atoi(retryAfter); err == nil && secs >= 0 {
		// Clamp before multiplying so a huge value cannot overflow Duration.
		wait = time.Duration(min(secs, int(maxPollRetryAfter/time.Second))) * time.Second
	}
	return min(wait, maxPollRetryAfter)
}

// signedRequest builds, signs, and performs one ACS request. It computes the
// content hash, sets x-ms-date and x-ms-content-sha256, and attaches the
// HMAC-SHA256 Authorization header over the request's path and query. extra
// headers are added verbatim.
func (c *Client) signedRequest(ctx context.Context, method, rawURL string, body []byte, extra http.Header) (*http.Response, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("acsemail: parse url: %w", err)
	}

	date := c.now().UTC().Format(http.TimeFormat)
	contentHash := computeContentHash(body)
	pathAndQuery := parsed.RequestURI()

	auth, err := signRequest(c.creds, method, pathAndQuery, parsed.Host, date, contentHash)
	if err != nil {
		return nil, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	// NewRequestWithContext copies the deadline into the request; the response
	// body must outlive this function, so cancel is deferred onto the response
	// via a wrapped close rather than here.
	req, err := http.NewRequestWithContext(reqCtx, method, rawURL, bytes.NewReader(body))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("acsemail: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-ms-date", date)
	req.Header.Set("x-ms-content-sha256", contentHash)
	req.Header.Set("Authorization", auth)
	for name, values := range extra {
		for _, v := range values {
			req.Header.Add(name, v)
		}
	}

	resp, err := c.http.Do(req)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("acsemail: request: %w", err)
	}
	resp.Body = &cancelOnCloseBody{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

// cancelOnCloseBody cancels the per-request context when the response body is
// closed, so the timeout context outlives the round-trip but is still released.
type cancelOnCloseBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelOnCloseBody) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}

// isSuccess reports whether an ACS operation status is the success terminal
// state. ACS uses "Succeeded"; some surfaces report "OK".
func isSuccess(status string) bool {
	return status == "Succeeded" || status == "OK"
}

// isTerminalFailure reports whether an ACS operation status is a failed terminal
// state (as opposed to the in-flight "Running"/"NotStarted" states).
func isTerminalFailure(status string) bool {
	switch status {
	case "Failed", "Canceled", "Cancelled":
		return true
	default:
		return false
	}
}

// compile-time assertions that both senders satisfy the consumer contract.
var (
	_ EmailSender = (*Client)(nil)
	_ EmailSender = (*NoOpSender)(nil)
)
