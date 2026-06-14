package acsemail

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
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
)

// ErrSendFailed reports that ACS accepted the request but the email operation
// finished in a non-success terminal state, or never reached success within the
// poll budget.
var ErrSendFailed = errors.New("acsemail: send failed")

// Message is the email a caller asks the client to deliver. The HTML body and
// subject are pre-built by the consumer (the digest worker, tc-34y5).
type Message struct {
	// Sender is the verified ACS sender address (e.g. hello@towncrierapp.uk).
	Sender string
	// Recipient is the single destination address.
	Recipient string
	// Subject is the email subject line.
	Subject string
	// HTMLBody is the rendered HTML email body.
	HTMLBody string
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
// A non-success terminal state, or exhausting the poll budget, returns an error.
func (c *Client) Send(ctx context.Context, msg Message) error {
	body, err := json.Marshal(sendRequest{
		SenderAddress: msg.Sender,
		Content:       sendContent{Subject: msg.Subject, HTML: msg.HTMLBody},
		Recipients:    sendRecipients{To: []sendAddress{{Address: msg.Recipient}}},
	})
	if err != nil {
		return fmt.Errorf("acsemail: marshal send request: %w", err)
	}

	opLocation, err := c.postSend(ctx, body)
	if err != nil {
		return err
	}

	return c.pollOperation(ctx, opLocation)
}

// postSend signs and POSTs the email send request, returning the
// Operation-Location URL to poll for completion.
func (c *Client) postSend(ctx context.Context, body []byte) (string, error) {
	endpoint := c.creds.endpoint + sendPath + "?api-version=" + apiVersion
	resp, err := c.signedRequest(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
	if err != nil {
		return "", fmt.Errorf("acsemail: read send response: %w", err)
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

		if err := sleep(ctx, c.pollInterval); err != nil {
			return err
		}
	}
	return fmt.Errorf("acsemail: %w: operation did not complete within %d polls", ErrSendFailed, c.maxPolls)
}

// getOperationStatus signs and GETs the operation-status URL.
func (c *Client) getOperationStatus(ctx context.Context, opLocation string) (operationStatus, error) {
	resp, err := c.signedRequest(ctx, http.MethodGet, opLocation, nil)
	if err != nil {
		return operationStatus{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
	if err != nil {
		return operationStatus{}, fmt.Errorf("acsemail: read operation status: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return operationStatus{}, fmt.Errorf("acsemail: %w: poll returned status %d: %s", ErrSendFailed, resp.StatusCode, string(respBody))
	}

	var status operationStatus
	if err := json.Unmarshal(respBody, &status); err != nil {
		return operationStatus{}, fmt.Errorf("acsemail: decode operation status: %w", err)
	}
	return status, nil
}

// signedRequest builds, signs, and performs one ACS request. It computes the
// content hash, sets x-ms-date and x-ms-content-sha256, and attaches the
// HMAC-SHA256 Authorization header over the request's path and query.
func (c *Client) signedRequest(ctx context.Context, method, rawURL string, body []byte) (*http.Response, error) {
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

// sleep waits for d or until ctx is cancelled, whichever comes first.
func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// compile-time assertions that both senders satisfy the consumer contract.
var (
	_ EmailSender = (*Client)(nil)
	_ EmailSender = (*NoOpSender)(nil)
)
