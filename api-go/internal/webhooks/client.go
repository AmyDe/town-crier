package webhooks

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/metric/noop"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

const (
	dialTimeout         = 3 * time.Second
	tlsHandshakeTimeout = 3 * time.Second
	responseTimeout     = 5 * time.Second
	requestTimeout      = 5 * time.Second
	maxResponseBytes    = 4 << 10
	redacted            = "redacted"
)

// FailureReason classifies a failed delivery. The values are stored and served.
type FailureReason string

// FailureReason values.
const (
	// FailureGone means the receiver answered 410.
	FailureGone FailureReason = "gone"
	// FailureHTTPStatus means the receiver answered another non-2xx status,
	// including a redirect.
	FailureHTTPStatus FailureReason = "http_status"
	// FailureBlockedAddress means the host resolved to an address or port that
	// webhooks may not connect to.
	FailureBlockedAddress FailureReason = "blocked_address"
	// FailureTimeout means the receiver did not answer in time.
	FailureTimeout FailureReason = "timeout"
	// FailureTLS means the TLS handshake failed.
	FailureTLS FailureReason = "tls"
	// FailureConnection means any other transport failure.
	FailureConnection FailureReason = "connection"
)

// Message is one request to a webhook. Body is sent byte for byte and signed
// with Secret.
type Message struct {
	URL       string
	Secret    string
	EventID   string
	EventType EventType
	Body      []byte
}

// Result is the outcome of one request. Failure is "" on success. Status is 0
// when no response arrived.
type Result struct {
	Failure FailureReason
	Status  int
}

// Delivered reports whether the receiver answered with a 2xx status.
func (r Result) Delivered() bool { return r.Failure == "" }

// Client sends signed requests to user-supplied URLs. It connects only to
// public addresses on port 443, follows no redirect, and keeps the URL and
// host out of its spans.
type Client struct {
	http *http.Client
	now  func() time.Time
}

// NewClient returns the production delivery client. now supplies the signed
// timestamp.
func NewClient(now func() time.Time) *Client {
	return newClient(newTransport(), now)
}

var errBlockedAddress = errors.New("webhooks: address is not allowed")

// dialControl runs on the resolved address of every connection attempt, so a
// host name that later resolves to a private address (DNS rebinding) is still
// refused.
func dialControl(_, address string, _ syscall.RawConn) error {
	addrPort, err := netip.ParseAddrPort(address)
	if err != nil || addrPort.Port() != 443 || !isPublicAddr(addrPort.Addr()) {
		return errBlockedAddress
	}
	return nil
}

// newTransport sets no Proxy: a proxy would make the connection itself and
// skip dialControl.
func newTransport() *http.Transport {
	dialer := &net.Dialer{Timeout: dialTimeout, Control: dialControl}
	return &http.Transport{
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   tlsHandshakeTimeout,
		ResponseHeaderTimeout: responseTimeout,
		MaxIdleConnsPerHost:   2,
		ForceAttemptHTTP2:     true,
	}
}

func newClient(base http.RoundTripper, now func() time.Time, opts ...otelhttp.Option) *Client {
	client := &http.Client{
		Timeout:       requestTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Transport: &platform.RedactingTransport{
			Next:   &opaqueTransport{next: base},
			Redact: func(*http.Request) string { return "https://" + redacted },
		},
	}
	// otelhttp also tags its metrics with server.address, which cannot be
	// rewritten, so this client records none.
	opts = append([]otelhttp.Option{otelhttp.WithMeterProvider(noop.NewMeterProvider())}, opts...)
	return &Client{
		http: platform.WrapHTTPClient(client, func(string, *http.Request) string { return "Webhook delivery" }, opts...),
		now:  now,
	}
}

// opaqueTransport removes the rest of what otelhttp would record about the
// receiver: the server.address attribute, and the text of transport and read
// errors, which names the host and becomes the span status.
type opaqueTransport struct {
	next http.RoundTripper
}

func (t *opaqueTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	trace.SpanFromContext(r.Context()).SetAttributes(semconv.ServerAddress(redacted))
	resp, err := t.next.RoundTrip(r)
	if err != nil {
		return nil, &deliveryError{reason: classifyError(r.Context(), err)}
	}
	resp.Body = &opaqueBody{ctx: r.Context(), ReadCloser: resp.Body}
	return resp, nil
}

type opaqueBody struct {
	io.ReadCloser
	ctx context.Context
}

func (b *opaqueBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		return n, &deliveryError{reason: classifyError(b.ctx, err)}
	}
	return n, err
}

type deliveryError struct {
	reason FailureReason
}

func (e *deliveryError) Error() string {
	return fmt.Sprintf("webhooks: delivery failed: %s", e.reason)
}

func classifyError(ctx context.Context, err error) FailureReason {
	var delivery *deliveryError
	var timeout interface{ Timeout() bool }
	switch {
	case errors.As(err, &delivery):
		return delivery.reason
	case errors.Is(err, errBlockedAddress):
		return FailureBlockedAddress
	case errors.Is(err, context.DeadlineExceeded),
		errors.Is(ctx.Err(), context.DeadlineExceeded),
		errors.As(err, &timeout) && timeout.Timeout():
		return FailureTimeout
	case isTLSError(err):
		return FailureTLS
	default:
		return FailureConnection
	}
}

func isTLSError(err error) bool {
	var certificate *tls.CertificateVerificationError
	var header tls.RecordHeaderError
	if errors.As(err, &certificate) || errors.As(err, &header) {
		return true
	}
	var op *net.OpError
	if errors.As(err, &op) && (op.Op == "remote error" || op.Op == "local error") {
		return true
	}
	// crypto/tls reports most handshake failures as untyped errors.
	return strings.HasPrefix(err.Error(), "tls: ")
}

// Send posts msg once and classifies the outcome. It returns no error, because
// a transport error quotes the URL. When ctx is cancelled the result is a
// failure, so a caller must check ctx before it counts one against a webhook.
func (c *Client) Send(ctx context.Context, msg Message) Result {
	req, err := newRequest(ctx, msg, c.now())
	if err != nil {
		return Result{Failure: FailureConnection}
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return Result{Failure: classifyError(ctx, err)}
	}
	defer func() { _ = resp.Body.Close() }()
	// Draining lets the connection be reused. The status alone decides the
	// outcome, so a read error changes nothing.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes)) //nolint:errcheck // best-effort drain

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode <= 299:
		return Result{Status: resp.StatusCode}
	case resp.StatusCode == http.StatusGone:
		return Result{Failure: FailureGone, Status: resp.StatusCode}
	default:
		return Result{Failure: FailureHTTPStatus, Status: resp.StatusCode}
	}
}
