package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/acsemail"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

var drainNow = time.Date(2026, 11, 26, 15, 0, 0, 0, time.UTC)

type fakeRow struct {
	Delivery
	status string
}

// fakeDrainStore keeps endpoints and deliveries in memory with the semantics of
// PostgresStore.
type fakeDrainStore struct {
	mu        sync.Mutex
	endpoints []*Endpoint
	rows      []*fakeRow

	loadErr, pendingErr, markErr, attemptErr, failureErr, successErr, disableErr error
}

func (f *fakeDrainStore) addEndpoint(id string, format Format) *Endpoint {
	e := &Endpoint{
		ID: id, UserID: "user-" + id, URL: "https://hooks.example.com/" + id,
		Format: format, Secret: testSecret, Status: StatusActive,
	}
	f.endpoints = append(f.endpoints, e)
	return e
}

func (f *fakeDrainStore) addRows(t *testing.T, endpointID string, ids ...string) {
	t.Helper()
	for _, id := range ids {
		n := testNotification(notifications.EventNewApplication, new(testZoneID), "Zone")
		app := testApplication()
		app.Name = "ref-" + id
		payload, err := BuildPayload(id, n, app, "Home", "leeds")
		if err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		f.rows = append(f.rows, &fakeRow{
			Delivery: Delivery{ID: id, EndpointID: endpointID, EventType: payload.Type, Payload: body},
			status:   "pending",
		})
	}
}

func (f *fakeDrainStore) row(id string) fakeRow {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.ID == id {
			return *r
		}
	}
	return fakeRow{}
}

func (f *fakeDrainStore) endpoint(id string) Endpoint {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range f.endpoints {
		if e.ID == id {
			return *e
		}
	}
	return Endpoint{}
}

func (f *fakeDrainStore) match(id, url string) *Endpoint {
	for _, e := range f.endpoints {
		if e.ID == id && e.URL == url {
			return e
		}
	}
	return nil
}

func (f *fakeDrainStore) ActiveEndpointsWithPending(context.Context) ([]Endpoint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.loadErr != nil {
		return nil, f.loadErr
	}
	var out []Endpoint
	for _, e := range f.endpoints {
		if e.Status == StatusActive && slices.ContainsFunc(f.rows, func(r *fakeRow) bool {
			return r.EndpointID == e.ID && r.status == "pending"
		}) {
			out = append(out, *e)
		}
	}
	return out, nil
}

func (f *fakeDrainStore) PendingDeliveries(_ context.Context, endpointID string, limit int) ([]Delivery, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pendingErr != nil {
		return nil, f.pendingErr
	}
	var out []Delivery
	for _, r := range f.rows {
		if r.EndpointID == endpointID && r.status == "pending" && len(out) < limit {
			out = append(out, r.Delivery)
		}
	}
	return out, nil
}

func (f *fakeDrainStore) MarkDelivered(ctx context.Context, ids []string, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := cmpErr(ctx.Err(), f.markErr); err != nil {
		return err
	}
	for _, r := range f.rows {
		if r.status == "pending" && slices.Contains(ids, r.ID) {
			r.status = "delivered"
			r.Attempts++
		}
	}
	return nil
}

func (f *fakeDrainStore) RecordFailedAttempt(ctx context.Context, ids []string, _ time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := cmpErr(ctx.Err(), f.attemptErr); err != nil {
		return 0, err
	}
	failed := 0
	for _, r := range f.rows {
		if r.status == "pending" && slices.Contains(ids, r.ID) {
			r.Attempts++
			if r.Attempts >= MaxDeliveryAttempts {
				r.status = "failed"
				failed++
			}
		}
	}
	return failed, nil
}

func (f *fakeDrainStore) RecordEndpointSuccess(ctx context.Context, id, url string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := cmpErr(ctx.Err(), f.successErr); err != nil {
		return err
	}
	if e := f.match(id, url); e != nil {
		e.ConsecutiveFailures = 0
		e.LastSuccessAt = &at
	}
	return nil
}

func (f *fakeDrainStore) RecordEndpointFailure(ctx context.Context, id, url string, res Result, at time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := cmpErr(ctx.Err(), f.failureErr); err != nil {
		return 0, err
	}
	e := f.match(id, url)
	if e == nil {
		return 0, ErrNotFound
	}
	e.ConsecutiveFailures++
	e.LastFailureAt = &at
	e.LastFailureReason = &res.Failure
	e.LastFailureStatus = &res.Status
	return e.ConsecutiveFailures, nil
}

func (f *fakeDrainStore) DisableEndpoint(ctx context.Context, id, url string, reason DisabledReason, _ time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := cmpErr(ctx.Err(), f.disableErr); err != nil {
		return false, err
	}
	e := f.match(id, url)
	if e == nil || e.Status != StatusActive {
		return false, nil
	}
	e.Status = StatusDisabled
	e.DisabledReason = &reason
	for _, r := range f.rows {
		if r.EndpointID == id && r.status == "pending" {
			r.status = "failed"
		}
	}
	return true, nil
}

func cmpErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

type fakeMessageSender struct {
	mu      sync.Mutex
	sent    []Message
	respond func(ctx context.Context, msg Message) Result
}

func (f *fakeMessageSender) Send(ctx context.Context, msg Message) Result {
	f.mu.Lock()
	f.sent = append(f.sent, msg)
	f.mu.Unlock()
	if f.respond == nil {
		return Result{Status: http.StatusOK}
	}
	return f.respond(ctx, msg)
}

func (f *fakeMessageSender) eventIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.sent))
	for i, m := range f.sent {
		out[i] = m.EventID
	}
	return out
}

func always(res Result) *fakeMessageSender {
	return &fakeMessageSender{respond: func(context.Context, Message) Result { return res }}
}

var (
	serverError = Result{Failure: FailureHTTPStatus, Status: 500}
	gone        = Result{Failure: FailureGone, Status: 410}
)

type fakeProfiles struct {
	profile *profiles.UserProfile
	err     error
}

func (f fakeProfiles) Get(context.Context, string) (*profiles.UserProfile, error) {
	return f.profile, f.err
}

func profileWithEmail(email *string) fakeProfiles {
	return fakeProfiles{profile: &profiles.UserProfile{Email: email}}
}

type sentEmail struct {
	kind string
	msg  acsemail.Message
}

type fakeEmailSender struct {
	mu     sync.Mutex
	sent   []sentEmail
	err    error
	ctxErr error
}

func (f *fakeEmailSender) Send(ctx context.Context, kind string, msg acsemail.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ctxErr = ctx.Err()
	f.sent = append(f.sent, sentEmail{kind, msg})
	return f.err
}

type drainFixture struct {
	store    *fakeDrainStore
	sender   *fakeMessageSender
	profiles fakeProfiles
	email    *fakeEmailSender
	logs     *syncBuffer
}

func newDrainFixture() *drainFixture {
	return &drainFixture{
		store:    &fakeDrainStore{},
		sender:   &fakeMessageSender{},
		profiles: profileWithEmail(new("owner@example.org")),
		email:    &fakeEmailSender{},
		logs:     &syncBuffer{},
	}
}

func (fx *drainFixture) drain(ctx context.Context) DrainResult {
	logger := slog.New(slog.NewJSONHandler(fx.logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return NewDrainer(fx.store, fx.sender, fx.profiles, fx.email, logger, func() time.Time { return drainNow }).Drain(ctx)
}

func assertRow(t *testing.T, store *fakeDrainStore, id, status string, attempts int) {
	t.Helper()
	if got := store.row(id); got.status != status || got.Attempts != attempts {
		t.Errorf("row %s = %s after %d attempts, want %s after %d", id, got.status, got.Attempts, status, attempts)
	}
}

func TestDrain_JSONSendsOldestFirstWithStoredBytes(t *testing.T) {
	t.Parallel()
	fx := newDrainFixture()
	e := fx.store.addEndpoint("e1", FormatJSON)
	e.ConsecutiveFailures = 5
	fx.store.addRows(t, "e1", "d1", "d2", "d3")

	got := fx.drain(t.Context())

	if want := (DrainResult{Endpoints: 1, Delivered: 3}); got != want {
		t.Errorf("result = %+v, want %+v", got, want)
	}
	if ids := fx.sender.eventIDs(); !slices.Equal(ids, []string{"d1", "d2", "d3"}) {
		t.Fatalf("sent = %v", ids)
	}
	for i, msg := range fx.sender.sent {
		row := fx.store.row(msg.EventID)
		if string(msg.Body) != string(row.Payload) || msg.URL != e.URL || msg.Secret != testSecret ||
			msg.EventType != EventApplicationNew {
			t.Errorf("message %d = %+v", i, msg.EventID)
		}
		assertRow(t, fx.store, msg.EventID, "delivered", 1)
	}
	if after := fx.store.endpoint("e1"); after.ConsecutiveFailures != 0 || after.LastSuccessAt == nil {
		t.Errorf("a success did not reset the endpoint: %d", after.ConsecutiveFailures)
	}
}

func TestDrain_JSONStopsAtTheFirstFailure(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		failAt       string
		wantSent     []string
		wantFailures int
		wantResult   DrainResult
	}{
		{"first send fails", "d1", []string{"d1"}, 3, DrainResult{Endpoints: 1, Failed: 1}},
		{"later send fails", "d2", []string{"d1", "d2"}, 0, DrainResult{Endpoints: 1, Delivered: 1, Failed: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fx := newDrainFixture()
			fx.store.addEndpoint("e1", FormatJSON).ConsecutiveFailures = 2
			fx.store.addRows(t, "e1", "d1", "d2", "d3")
			fx.sender.respond = func(_ context.Context, m Message) Result {
				if m.EventID == tc.failAt {
					return serverError
				}
				return Result{Status: 200}
			}

			got := fx.drain(t.Context())

			if got != tc.wantResult {
				t.Errorf("result = %+v, want %+v", got, tc.wantResult)
			}
			if ids := fx.sender.eventIDs(); !slices.Equal(ids, tc.wantSent) {
				t.Errorf("sent = %v, want %v", ids, tc.wantSent)
			}
			assertRow(t, fx.store, tc.failAt, "pending", 1)
			assertRow(t, fx.store, "d3", "pending", 0)
			if after := fx.store.endpoint("e1"); after.ConsecutiveFailures != tc.wantFailures || after.Status != StatusActive {
				t.Errorf("consecutive failures = %d, want %d", after.ConsecutiveFailures, tc.wantFailures)
			}
		})
	}
}

func TestDrain_SixthFailedAttemptGivesUpTheRow(t *testing.T) {
	t.Parallel()
	fx := newDrainFixture()
	fx.store.addEndpoint("e1", FormatJSON)
	fx.store.addRows(t, "e1", "d1", "d2")
	fx.sender = always(serverError)

	for attempt := 1; attempt <= 6; attempt++ {
		fx.drain(t.Context())
		want := "pending"
		if attempt == 6 {
			want = "failed"
		}
		assertRow(t, fx.store, "d1", want, attempt)
	}
	assertRow(t, fx.store, "d2", "pending", 0)

	fx.drain(t.Context())
	assertRow(t, fx.store, "d2", "pending", 1)
}

func TestDrain_ReadsAtMost100RowsPerEndpoint(t *testing.T) {
	t.Parallel()
	for _, format := range []Format{FormatJSON, FormatSlack} {
		t.Run(string(format), func(t *testing.T) {
			t.Parallel()
			fx := newDrainFixture()
			fx.store.addEndpoint("e1", format)
			for i := range 101 {
				fx.store.addRows(t, "e1", fmt.Sprintf("d%03d", i))
			}

			if got := fx.drain(t.Context()); got.Delivered != 100 {
				t.Errorf("delivered = %d, want 100", got.Delivered)
			}
			assertRow(t, fx.store, "d100", "pending", 0)
		})
	}
}

func slackSentText(t *testing.T, msg Message) string {
	t.Helper()
	var body struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		t.Fatal(err)
	}
	return body.Text
}

func TestDrain_SlackSendsOneSummary(t *testing.T) {
	t.Parallel()
	fx := newDrainFixture()
	fx.store.addEndpoint("e1", FormatSlack)
	var ids []string
	for i := range 13 {
		ids = append(ids, fmt.Sprintf("d%02d", i))
	}
	fx.store.addRows(t, "e1", ids...)

	got := fx.drain(t.Context())

	if got != (DrainResult{Endpoints: 1, Delivered: 13}) {
		t.Errorf("result = %+v", got)
	}
	if len(fx.sender.sent) != 1 {
		t.Fatalf("messages = %d, want 1", len(fx.sender.sent))
	}
	msg := fx.sender.sent[0]
	if msg.EventID != "d00" || msg.EventType != EventApplicationNew {
		t.Errorf("event = %s %s, want the oldest row", msg.EventID, msg.EventType)
	}
	text := slackSentText(t, msg)
	if !strings.HasPrefix(text, "*Town Crier: 13 planning updates*\n") ||
		!strings.HasSuffix(text, "\n…and 3 more in the Town Crier app") || strings.Count(text, "• ") != 10 ||
		strings.Index(text, "ref-d00") > strings.Index(text, "ref-d09") {
		t.Errorf("text = %q", text)
	}
	for _, id := range ids {
		assertRow(t, fx.store, id, "delivered", 1)
	}
}

func TestDrain_SlackFailureCountsEveryRow(t *testing.T) {
	t.Parallel()
	fx := newDrainFixture()
	fx.store.addEndpoint("e1", FormatSlack)
	fx.store.addRows(t, "e1", "d1", "d2")
	fx.store.rows[0].Attempts = 5
	fx.sender = always(serverError)

	got := fx.drain(t.Context())

	if got != (DrainResult{Endpoints: 1, Failed: 2}) {
		t.Errorf("result = %+v", got)
	}
	assertRow(t, fx.store, "d1", "failed", 6)
	assertRow(t, fx.store, "d2", "pending", 1)
	if n := fx.store.endpoint("e1").ConsecutiveFailures; n != 1 {
		t.Errorf("consecutive failures = %d, want 1 per run", n)
	}
}

func TestDrain_SlackUnreadablePayloadCountsAsFailedAttempt(t *testing.T) {
	t.Parallel()
	fx := newDrainFixture()
	fx.store.addEndpoint("e1", FormatSlack)
	fx.store.addRows(t, "e1", "bad", "good")
	fx.store.rows[0].Payload = []byte(`{"schemaVersion":"one"}`)

	got := fx.drain(t.Context())

	if got != (DrainResult{Endpoints: 1, Delivered: 1, Failed: 1}) {
		t.Errorf("result = %+v", got)
	}
	assertRow(t, fx.store, "bad", "pending", 1)
	assertRow(t, fx.store, "good", "delivered", 1)
	if len(fx.sender.sent) != 1 || fx.sender.sent[0].EventID != "good" {
		t.Errorf("sent = %v", fx.sender.eventIDs())
	}
}

func TestDrain_GoneDisablesAtOnce(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		format Format
		goneAt string
	}{
		{"first json send", FormatJSON, "d1"},
		{"json send after a success", FormatJSON, "d2"},
		{"slack summary", FormatSlack, "d1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fx := newDrainFixture()
			fx.store.addEndpoint("e1", tc.format)
			fx.store.addRows(t, "e1", "d1", "d2", "d3")
			fx.sender.respond = func(_ context.Context, m Message) Result {
				if m.EventID == tc.goneAt {
					return gone
				}
				return Result{Status: 200}
			}

			got := fx.drain(t.Context())

			if got.Disabled != 1 {
				t.Errorf("result = %+v", got)
			}
			after := fx.store.endpoint("e1")
			if after.Status != StatusDisabled || after.DisabledReason == nil || *after.DisabledReason != DisabledGone {
				t.Errorf("status = %s", after.Status)
			}
			wantAttempts := 0
			if tc.format == FormatSlack {
				wantAttempts = 1
			}
			assertRow(t, fx.store, "d3", "failed", wantAttempts)
			if len(fx.email.sent) != 1 || !strings.Contains(fx.email.sent[0].msg.HTMLBody, "no longer exists") {
				t.Errorf("emails = %d", len(fx.email.sent))
			}
		})
	}
}

func TestDrain_24thConsecutiveFailedRunDisablesAndEmailsOnce(t *testing.T) {
	t.Parallel()
	fx := newDrainFixture()
	fx.store.addEndpoint("e1", FormatJSON).ConsecutiveFailures = 22
	fx.store.addRows(t, "e1", "d1", "d2", "d3")
	fx.sender = always(serverError)

	if got := fx.drain(t.Context()); got.Disabled != 0 || len(fx.email.sent) != 0 {
		t.Fatalf("23rd run: %+v, %d emails", got, len(fx.email.sent))
	}
	got := fx.drain(t.Context())
	fx.drain(t.Context())

	if got != (DrainResult{Endpoints: 1, Failed: 1, Disabled: 1}) {
		t.Errorf("24th run = %+v", got)
	}
	after := fx.store.endpoint("e1")
	if after.Status != StatusDisabled || *after.DisabledReason != DisabledFailures {
		t.Errorf("status = %s", after.Status)
	}
	assertRow(t, fx.store, "d1", "failed", 2)
	assertRow(t, fx.store, "d2", "failed", 0)
	assertRow(t, fx.store, "d3", "failed", 0)
	if len(fx.email.sent) != 1 {
		t.Fatalf("emails = %d, want 1", len(fx.email.sent))
	}
	sent := fx.email.sent[0]
	if sent.kind != "webhook-disabled" || sent.msg.Recipient != "owner@example.org" {
		t.Errorf("email = %s to %s", sent.kind, sent.msg.Recipient)
	}
}

func TestDrain_SwitchOffEmailNeedsAnAddress(t *testing.T) {
	t.Parallel()
	for name, reader := range map[string]fakeProfiles{
		"nil email":     profileWithEmail(nil),
		"blank email":   profileWithEmail(new("  ")),
		"no profile":    {err: profiles.ErrNotFound},
		"nil profile":   {},
		"profile error": {err: errors.New("database down")},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fx := newDrainFixture()
			fx.profiles = reader
			fx.store.addEndpoint("e1", FormatJSON)
			fx.store.addRows(t, "e1", "d1")
			fx.sender = always(gone)

			if got := fx.drain(t.Context()); got.Disabled != 1 {
				t.Errorf("result = %+v", got)
			}
			if len(fx.email.sent) != 0 {
				t.Errorf("emails = %d, want none", len(fx.email.sent))
			}
		})
	}
}

// alreadyDisabled is a store where another call switched the endpoint off first.
type alreadyDisabled struct{ *fakeDrainStore }

func (alreadyDisabled) DisableEndpoint(context.Context, string, string, DisabledReason, time.Time) (bool, error) {
	return false, nil
}

func TestDrain_NoEmailUnlessThisCallDisabled(t *testing.T) {
	t.Parallel()
	fx := newDrainFixture()
	fx.store.addEndpoint("e1", FormatJSON)
	fx.store.addRows(t, "e1", "d1")
	email := &fakeEmailSender{}
	drainer := NewDrainer(alreadyDisabled{fx.store}, always(gone), fx.profiles, email,
		slog.New(slog.DiscardHandler), time.Now)

	if got := drainer.Drain(t.Context()); got.Disabled != 0 || len(email.sent) != 0 {
		t.Errorf("result = %+v, emails = %d", got, len(email.sent))
	}
}

func TestDrain_EmailFailureDoesNotUndoTheSwitchOff(t *testing.T) {
	t.Parallel()
	for _, sendErr := range []error{errors.New("acs rejected"), acsemail.ErrAcceptedStatusUnknown} {
		fx := newDrainFixture()
		fx.email.err = sendErr
		fx.store.addEndpoint("e1", FormatJSON)
		fx.store.addRows(t, "e1", "d1")
		fx.sender = always(gone)

		if got := fx.drain(t.Context()); got.Disabled != 1 || fx.store.endpoint("e1").Status != StatusDisabled {
			t.Errorf("%v: result = %+v", sendErr, got)
		}
	}
}

func TestDrain_SwitchOffEmailOutlivesTheDrainDeadline(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	fx := newDrainFixture()
	fx.store.addEndpoint("e1", FormatJSON)
	fx.store.addRows(t, "e1", "d1", "d2")
	fx.sender.respond = func(_ context.Context, m Message) Result {
		if m.EventID == "d1" {
			return Result{Status: 200}
		}
		return gone
	}
	cancelling := &cancelOnDisable{fx.store, cancel}
	NewDrainer(cancelling, fx.sender, fx.profiles, fx.email, slog.New(slog.DiscardHandler), time.Now).Drain(ctx)

	if len(fx.email.sent) != 1 || fx.email.ctxErr != nil {
		t.Errorf("emails = %d, send context error = %v", len(fx.email.sent), fx.email.ctxErr)
	}
}

type cancelOnDisable struct {
	*fakeDrainStore
	cancel context.CancelFunc
}

func (c *cancelOnDisable) DisableEndpoint(ctx context.Context, id, url string, reason DisabledReason, at time.Time) (bool, error) {
	disabled, err := c.fakeDrainStore.DisableEndpoint(ctx, id, url, reason, at)
	c.cancel()
	return disabled, err
}

func TestDrain_CancelledContextCountsNothing(t *testing.T) {
	t.Parallel()
	for _, format := range []Format{FormatJSON, FormatSlack} {
		t.Run(string(format), func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			fx := newDrainFixture()
			fx.store.addEndpoint("e1", format).ConsecutiveFailures = 23
			fx.store.addRows(t, "e1", "d1", "d2")
			fx.sender.respond = func(context.Context, Message) Result {
				cancel()
				return Result{Failure: FailureConnection}
			}

			got := fx.drain(ctx)

			if got != (DrainResult{Endpoints: 1}) {
				t.Errorf("result = %+v", got)
			}
			assertRow(t, fx.store, "d1", "pending", 0)
			assertRow(t, fx.store, "d2", "pending", 0)
			if after := fx.store.endpoint("e1"); after.ConsecutiveFailures != 23 || after.Status != StatusActive ||
				after.LastFailureAt != nil {
				t.Errorf("endpoint changed: %d %s", after.ConsecutiveFailures, after.Status)
			}
		})
	}
}

func TestDrain_DeliveriesBeforeACancelAreStillRecorded(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	fx := newDrainFixture()
	fx.store.addEndpoint("e1", FormatJSON).ConsecutiveFailures = 3
	fx.store.addRows(t, "e1", "d1", "d2")
	fx.sender.respond = func(context.Context, Message) Result {
		cancel()
		return Result{Status: 200}
	}

	got := fx.drain(ctx)

	if got != (DrainResult{Endpoints: 1, Delivered: 1}) || len(fx.sender.sent) != 1 {
		t.Errorf("result = %+v, sends = %d", got, len(fx.sender.sent))
	}
	assertRow(t, fx.store, "d1", "delivered", 1)
	assertRow(t, fx.store, "d2", "pending", 0)
	if n := fx.store.endpoint("e1").ConsecutiveFailures; n != 0 {
		t.Errorf("consecutive failures = %d, want 0", n)
	}
}

func TestDrain_ReplacedURLIsNotPenalised(t *testing.T) {
	t.Parallel()
	for name, res := range map[string]Result{"gone": gone, "24th failure": serverError} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fx := newDrainFixture()
			e := fx.store.addEndpoint("e1", FormatJSON)
			e.ConsecutiveFailures = 23
			fx.store.addRows(t, "e1", "d1")
			fx.sender.respond = func(context.Context, Message) Result {
				fx.store.mu.Lock()
				defer fx.store.mu.Unlock()
				e.URL, e.ConsecutiveFailures = "https://new.example.com/hook", 0
				return res
			}

			got := fx.drain(t.Context())

			if got.Disabled != 0 || len(fx.email.sent) != 0 {
				t.Errorf("result = %+v, emails = %d", got, len(fx.email.sent))
			}
			after := fx.store.endpoint("e1")
			if after.Status != StatusActive || after.ConsecutiveFailures != 0 || after.LastFailureAt != nil {
				t.Errorf("replaced endpoint changed: %s %d", after.Status, after.ConsecutiveFailures)
			}
		})
	}
}

func TestDrain_RunsAtMostFourEndpointsAtOnce(t *testing.T) {
	t.Parallel()
	fx := newDrainFixture()
	for i := range 9 {
		id := fmt.Sprintf("e%d", i)
		fx.store.addEndpoint(id, FormatJSON)
		fx.store.addRows(t, id, id+"-a", id+"-b")
	}
	var mu sync.Mutex
	inFlight, peak := 0, 0
	perEndpoint := map[string]int{}
	overlapped := false
	fourBusy := make(chan struct{})
	var once sync.Once
	fx.sender.respond = func(_ context.Context, m Message) Result {
		mu.Lock()
		inFlight++
		peak = max(peak, inFlight)
		perEndpoint[m.URL]++
		overlapped = overlapped || perEndpoint[m.URL] > 1
		if inFlight == 4 {
			once.Do(func() { close(fourBusy) })
		}
		mu.Unlock()
		select {
		case <-fourBusy:
		case <-time.After(2 * time.Second):
		}
		mu.Lock()
		inFlight--
		perEndpoint[m.URL]--
		mu.Unlock()
		return Result{Status: 200}
	}

	got := fx.drain(t.Context())

	if got != (DrainResult{Endpoints: 9, Delivered: 18}) {
		t.Errorf("result = %+v", got)
	}
	if peak != 4 || overlapped {
		t.Errorf("peak = %d, overlap within an endpoint = %v", peak, overlapped)
	}
}

func TestDrain_StoreErrors(t *testing.T) {
	t.Parallel()
	down := errors.New("database down")
	for _, tc := range []struct {
		name      string
		store     fakeDrainStore
		result    Result
		wantSends int
	}{
		{"load", fakeDrainStore{loadErr: down}, Result{Status: 200}, 0},
		{"pending", fakeDrainStore{pendingErr: down}, Result{Status: 200}, 0},
		{"mark delivered stops the endpoint", fakeDrainStore{markErr: down}, Result{Status: 200}, 1},
		{"record attempt", fakeDrainStore{attemptErr: down}, serverError, 1},
		{"record success", fakeDrainStore{successErr: down}, Result{Status: 200}, 2},
		{"record failure", fakeDrainStore{failureErr: down}, serverError, 1},
		{"disable", fakeDrainStore{disableErr: down}, gone, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fx := newDrainFixture()
			fx.store = &tc.store
			fx.store.addEndpoint("e1", FormatJSON)
			fx.store.addRows(t, "e1", "d1", "d2")
			fx.sender = always(tc.result)

			got := fx.drain(t.Context())

			if len(fx.sender.sent) != tc.wantSends || got.Disabled != 0 {
				t.Errorf("sends = %d, result = %+v", len(fx.sender.sent), got)
			}
			if !strings.Contains(fx.logs.String(), `"level":"ERROR"`) {
				t.Errorf("no error was logged: %s", fx.logs.String())
			}
		})
	}
}

func TestDrain_LogsOneSummaryLine(t *testing.T) {
	t.Parallel()
	fx := newDrainFixture()
	fx.store.addEndpoint("e1", FormatJSON)
	fx.store.addRows(t, "e1", "d1", "d2")

	fx.drain(t.Context())

	var summaries []map[string]any
	dec := json.NewDecoder(strings.NewReader(fx.logs.String()))
	for {
		var line map[string]any
		if err := dec.Decode(&line); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		if line["msg"] == "webhook.drain" {
			summaries = append(summaries, line)
		}
	}
	if len(summaries) != 1 {
		t.Fatalf("summary lines = %d", len(summaries))
	}
	for key, want := range map[string]float64{"endpoints": 1, "delivered": 2, "failed": 0, "disabled": 0} {
		if summaries[0][key] != want {
			t.Errorf("%s = %v, want %v", key, summaries[0][key], want)
		}
	}
}

type syncBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}
