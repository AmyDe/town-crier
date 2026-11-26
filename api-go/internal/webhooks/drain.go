package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/acsemail"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

const (
	drainParallelism    = 4
	maxDeliveriesPerRun = 100
	// Writes and the switch-off email run detached from the drain's context, so
	// each needs its own limit.
	storeWriteTimeout   = 5 * time.Second
	disabledSendTimeout = 30 * time.Second
)

type drainStore interface {
	ActiveEndpointsWithPending(ctx context.Context) ([]Endpoint, error)
	PendingDeliveries(ctx context.Context, endpointID string, limit int) ([]Delivery, error)
	MarkDelivered(ctx context.Context, ids []string, at time.Time) error
	RecordFailedAttempt(ctx context.Context, ids []string, at time.Time) (int, error)
	RecordEndpointSuccess(ctx context.Context, endpointID, url string, at time.Time) error
	RecordEndpointFailure(ctx context.Context, endpointID, url string, result Result, at time.Time) (int, error)
	DisableEndpoint(ctx context.Context, endpointID, url string, reason DisabledReason, at time.Time) (bool, error)
}

type messageSender interface {
	Send(ctx context.Context, msg Message) Result
}

type profileReader interface {
	Get(ctx context.Context, userID string) (*profiles.UserProfile, error)
}

type kindEmailSender interface {
	Send(ctx context.Context, kind string, msg acsemail.Message) error
}

// DrainResult counts one drain run: the endpoints that had pending alerts, the
// deliveries accepted by a receiver, the deliveries whose attempt failed, and
// the endpoints switched off.
type DrainResult struct {
	Endpoints int
	Delivered int
	Failed    int
	Disabled  int
}

// Drainer sends queued deliveries to their endpoints.
type Drainer struct {
	store    drainStore
	sender   messageSender
	profiles profileReader
	email    kindEmailSender
	logger   *slog.Logger
	now      func() time.Time
}

// NewDrainer returns a drainer. In production store is a *PostgresStore, sender
// a *Client and email an *acsemail.InstrumentedSender.
func NewDrainer(store drainStore, sender messageSender, profiles profileReader, email kindEmailSender, logger *slog.Logger, now func() time.Time) *Drainer {
	return &Drainer{store: store, sender: sender, profiles: profiles, email: email, logger: logger, now: now}
}

// Drain makes one delivery run over every active endpoint with pending alerts,
// up to 4 endpoints at a time and one request at a time per endpoint. It
// applies the retry and switch-off rules and logs failures, so it returns no
// error. Once ctx is done it starts no request and counts no failure: untried
// deliveries stay pending. It must not run concurrently with itself.
func (d *Drainer) Drain(ctx context.Context) DrainResult {
	endpoints, err := d.store.ActiveEndpointsWithPending(ctx)
	if err != nil {
		d.logger.ErrorContext(ctx, "webhook drain: load endpoints failed", "error", err)
	}

	total := DrainResult{Endpoints: len(endpoints)}
	var mu sync.Mutex
	var wg sync.WaitGroup
	slots := make(chan struct{}, drainParallelism)
	for _, e := range endpoints {
		slots <- struct{}{}
		wg.Go(func() {
			defer func() { <-slots }()
			run := d.drainEndpoint(ctx, e)
			mu.Lock()
			defer mu.Unlock()
			total.Delivered += run.delivered
			total.Failed += run.failed
			if run.disabled {
				total.Disabled++
			}
		})
	}
	wg.Wait()

	d.logger.InfoContext(ctx, "webhook.drain",
		"endpoints", total.Endpoints, "delivered", total.Delivered, "failed", total.Failed, "disabled", total.Disabled)
	return total
}

// endpointRun is what one run did to one endpoint. failure is the request
// failure that ended it, if any.
type endpointRun struct {
	delivered int
	failed    int
	failure   *Result
	disabled  bool
}

func (d *Drainer) drainEndpoint(ctx context.Context, e Endpoint) endpointRun {
	var run endpointRun
	if ctx.Err() != nil {
		return run
	}
	pending, err := d.store.PendingDeliveries(ctx, e.ID, maxDeliveriesPerRun)
	if err != nil {
		d.logError(ctx, "webhook drain: load deliveries failed", e.ID, err)
		return run
	}
	if e.Format == FormatSlack {
		d.sendSlack(ctx, e, pending, &run)
	} else {
		d.sendJSON(ctx, e, pending, &run)
	}
	d.recordRun(ctx, e, &run)
	return run
}

func (d *Drainer) sendJSON(ctx context.Context, e Endpoint, pending []Delivery, run *endpointRun) {
	for _, delivery := range pending {
		msg := Message{
			URL: e.URL, Secret: e.Secret,
			EventID: delivery.ID, EventType: delivery.EventType, Body: delivery.Payload,
		}
		if !d.attempt(ctx, e.ID, msg, []string{delivery.ID}, run) {
			return
		}
	}
}

func (d *Drainer) sendSlack(ctx context.Context, e Endpoint, pending []Delivery, run *endpointRun) {
	payloads := make([]Payload, 0, len(pending))
	sent := make([]Delivery, 0, len(pending))
	var unreadable []string
	for _, delivery := range pending {
		var p Payload
		if err := json.Unmarshal(delivery.Payload, &p); err != nil {
			d.logError(ctx, "webhook drain: unreadable payload", e.ID, err)
			unreadable = append(unreadable, delivery.ID)
			continue
		}
		payloads = append(payloads, p)
		sent = append(sent, delivery)
	}
	if len(unreadable) > 0 {
		d.recordFailedAttempt(ctx, e.ID, unreadable, run)
	}
	if len(sent) == 0 {
		return
	}
	body, err := SlackBody(payloads)
	if err != nil {
		d.logError(ctx, "webhook drain: build slack message failed", e.ID, err)
		return
	}
	ids := make([]string, len(sent))
	for i, delivery := range sent {
		ids[i] = delivery.ID
	}
	d.attempt(ctx, e.ID, Message{
		URL: e.URL, Secret: e.Secret,
		EventID: sent[0].ID, EventType: sent[0].EventType, Body: body,
	}, ids, run)
}

// attempt sends one request that covers the deliveries in ids and records the
// outcome on them. It reports whether the endpoint's run can continue.
func (d *Drainer) attempt(ctx context.Context, endpointID string, msg Message, ids []string, run *endpointRun) bool {
	if ctx.Err() != nil {
		return false
	}
	result := d.sender.Send(ctx, msg)
	if !result.Delivered() {
		// A cancelled send also fails, and says nothing about the receiver.
		if ctx.Err() != nil {
			return false
		}
		d.logger.WarnContext(ctx, "webhook delivery failed",
			"endpointId", endpointID, "reason", string(result.Failure), "status", result.Status)
		run.failure = &result
		d.recordFailedAttempt(ctx, endpointID, ids, run)
		return false
	}
	run.delivered += len(ids)
	// The receiver has the alert, so the record of it must survive a cancel.
	writeCtx, cancel := detached(ctx, storeWriteTimeout)
	defer cancel()
	if err := d.store.MarkDelivered(writeCtx, ids, d.now().UTC()); err != nil {
		// Stopping limits how many alerts the next run sends again.
		d.logError(ctx, "webhook drain: mark delivered failed", endpointID, err)
		return false
	}
	return true
}

func (d *Drainer) recordFailedAttempt(ctx context.Context, endpointID string, ids []string, run *endpointRun) {
	run.failed += len(ids)
	writeCtx, cancel := detached(ctx, storeWriteTimeout)
	defer cancel()
	if _, err := d.store.RecordFailedAttempt(writeCtx, ids, d.now().UTC()); err != nil {
		d.logError(ctx, "webhook drain: record attempt failed", endpointID, err)
	}
}

// recordRun updates the endpoint once per run: any delivery clears its failure
// count, and a run whose first request failed adds one. Every store write names
// the URL the run sent to, so an endpoint that was given a new URL meanwhile is
// left alone.
func (d *Drainer) recordRun(ctx context.Context, e Endpoint, run *endpointRun) {
	writeCtx, cancel := detached(ctx, storeWriteTimeout)
	defer cancel()
	failures := 0
	switch {
	case run.delivered > 0:
		if err := d.store.RecordEndpointSuccess(writeCtx, e.ID, e.URL, d.now().UTC()); err != nil {
			d.logError(ctx, "webhook drain: record success failed", e.ID, err)
		}
	case run.failure != nil:
		count, err := d.store.RecordEndpointFailure(writeCtx, e.ID, e.URL, *run.failure, d.now().UTC())
		if errors.Is(err, ErrNotFound) {
			return
		}
		if err != nil {
			d.logError(ctx, "webhook drain: record failure failed", e.ID, err)
		}
		failures = count
	}
	if run.failure == nil {
		return
	}
	reason, off := SwitchOffReason(run.failure.Failure, failures)
	if !off {
		return
	}
	at := d.now().UTC()
	disabled, err := d.store.DisableEndpoint(writeCtx, e.ID, e.URL, reason, at)
	if err != nil {
		d.logError(ctx, "webhook drain: switch off failed", e.ID, err)
		return
	}
	if !disabled {
		return
	}
	run.disabled = true
	d.logger.InfoContext(ctx, "webhook switched off", "endpointId", e.ID, "reason", string(reason))
	d.emailOwner(ctx, e, reason, at)
}

// emailOwner is the only notice of a switch-off, which happens once, so a done
// drain context does not stop it.
func (d *Drainer) emailOwner(ctx context.Context, e Endpoint, reason DisabledReason, at time.Time) {
	sendCtx, cancel := detached(ctx, disabledSendTimeout)
	defer cancel()
	profile, err := d.profiles.Get(sendCtx, e.UserID)
	if errors.Is(err, profiles.ErrNotFound) {
		return
	}
	if err != nil {
		d.logError(ctx, "webhook drain: read profile failed", e.ID, err)
		return
	}
	if profile == nil || profile.Email == nil || strings.TrimSpace(*profile.Email) == "" {
		return
	}
	err = d.email.Send(sendCtx, emailKindDisabled, disabledEmail(*profile.Email, e.ID, reason, at))
	if err != nil && !errors.Is(err, acsemail.ErrAcceptedStatusUnknown) {
		d.logError(ctx, "webhook drain: switched-off email failed", e.ID, err)
	}
}

func (d *Drainer) logError(ctx context.Context, msg, endpointID string, err error) {
	d.logger.ErrorContext(ctx, msg, "endpointId", endpointID, "error", err) //nolint:sloglint // msg is a constant at every call site
}

func detached(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), timeout)
}
