package subscriptionevents

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

const (
	deliveryTimeout   = 10 * time.Second
	httpClientTimeout = 5 * time.Second
)

// TotalsReader supplies the counts behind Totals.
type TotalsReader interface {
	UserStats(ctx context.Context, now time.Time) (profiles.UserStats, error)
	PaidCandidates(ctx context.Context) ([]*profiles.UserProfile, error)
}

// Notifier turns subscription snapshot changes into webhook deliveries.
type Notifier struct {
	totals   TotalsReader
	webhooks []Webhook
	client   *http.Client
	logger   *slog.Logger
	now      func() time.Time
	wg       sync.WaitGroup
}

// NewNotifier builds a notifier. client should come from NewHTTPClient.
func NewNotifier(totals TotalsReader, webhooks []Webhook, client *http.Client, logger *slog.Logger, now func() time.Time) *Notifier {
	return &Notifier{totals: totals, webhooks: webhooks, client: client, logger: logger, now: now}
}

// NewHTTPClient returns the 5-second, otel-instrumented client used for delivery.
func NewHTTPClient() *http.Client {
	return newHTTPClient(http.DefaultTransport)
}

func newHTTPClient(base http.RoundTripper, opts ...otelhttp.Option) *http.Client {
	client := &http.Client{Timeout: httpClientTimeout}
	// Innermost, so it overwrites the span's url.full (webhook URLs carry tokens) after otelhttp starts the span.
	client.Transport = &platform.RedactingTransport{Next: base, Redact: redactWebhookURL}
	return platform.WrapHTTPClient(client, func(string, *http.Request) string {
		return "Subscription webhook"
	}, opts...)
}

func redactWebhookURL(r *http.Request) string {
	return r.URL.Scheme + "://" + r.URL.Host
}

// Changed reports a saved change. It returns immediately: when the snapshots
// classify as an event and webhooks are configured, totals are computed and the
// payload delivered in a background goroutine that outlives ctx. A nil
// Notifier, no webhooks, or no event is a no-op.
func (n *Notifier) Changed(ctx context.Context, before, after Snapshot, source Source) {
	if n == nil || len(n.webhooks) == 0 {
		return
	}
	typ, ok := Classify(before, after)
	if !ok {
		return
	}
	occurredAt := n.now()
	bgCtx := context.WithoutCancel(ctx)

	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		ctx, cancel := context.WithTimeout(bgCtx, deliveryTimeout)
		defer cancel()
		n.deliver(ctx, typ, before, after, source, occurredAt)
	}()
}

// Wait blocks until in-flight deliveries finish. Short-lived jobs call it
// before exiting; the API does not.
func (n *Notifier) Wait() {
	if n == nil {
		return
	}
	n.wg.Wait()
}

func (n *Notifier) deliver(ctx context.Context, typ EventType, before, after Snapshot, source Source, occurredAt time.Time) {
	stats, err := n.totals.UserStats(ctx, occurredAt)
	if err != nil {
		n.logger.WarnContext(ctx, "subscription webhook totals failed", "eventType", string(typ))
		return
	}
	candidates, err := n.totals.PaidCandidates(ctx)
	if err != nil {
		n.logger.WarnContext(ctx, "subscription webhook totals failed", "eventType", string(typ))
		return
	}
	payload := BuildPayload(newEventID(), typ, before, after, source, occurredAt, ComputeTotals(stats.Total, candidates, occurredAt))
	raw, err := json.Marshal(payload)
	if err != nil {
		n.logger.ErrorContext(ctx, "subscription webhook payload failed", "eventType", string(typ))
		return
	}

	var wg sync.WaitGroup
	for i, hook := range n.webhooks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n.post(ctx, i, hook, typ, raw)
		}()
	}
	wg.Wait()
}

// post sends one webhook. Failures log only the index, status and event type:
// the URL and headers are secrets, and a transport error would embed the URL.
func (n *Notifier) post(ctx context.Context, index int, hook Webhook, typ EventType, payload []byte) {
	body := payload
	if hook.BodyField != "" {
		wrapped, err := json.Marshal(map[string]string{hook.BodyField: string(payload)})
		if err != nil {
			n.logger.ErrorContext(ctx, "subscription webhook body failed", "webhookIndex", index, "eventType", string(typ))
			return
		}
		body = wrapped
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hook.URL, bytes.NewReader(body))
	if err != nil {
		n.logger.WarnContext(ctx, "subscription webhook request invalid", "webhookIndex", index, "eventType", string(typ))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range hook.Headers {
		req.Header.Set(k, v)
	}
	resp, err := n.client.Do(req)
	if err != nil {
		n.logger.WarnContext(ctx, "subscription webhook delivery failed", "webhookIndex", index, "eventType", string(typ))
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		n.logger.WarnContext(ctx, "subscription webhook rejected", "webhookIndex", index, "status", resp.StatusCode, "eventType", string(typ))
	}
}
