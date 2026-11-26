package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/AmyDe/town-crier/api-go/internal/notifications"
)

type fakeExecer struct {
	args [][]any
	err  error
}

func (f *fakeExecer) Exec(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
	f.args = append(f.args, args)
	return pgconn.NewCommandTag("INSERT 0 0"), f.err
}

type fakeSlugs map[int]string

func (f fakeSlugs) SlugForAreaID(id int) (string, bool) {
	slug, ok := f[id]
	return slug, ok
}

func TestOutbox_Enqueue(t *testing.T) {
	t.Parallel()
	db := &fakeExecer{}
	now := time.Date(2026, 11, 26, 14, 5, 0, 0, time.UTC)
	outbox := NewOutbox(db, fakeSlugs{100: "leeds"}, func() string { return testEventID }, func() time.Time { return now })
	n := testNotification(notifications.EventDecisionUpdate, new(testZoneID), "Zone")

	if err := outbox.Enqueue(t.Context(), n, testApplication(), "Home"); err != nil {
		t.Fatal(err)
	}

	if len(db.args) != 1 || len(db.args[0]) != 6 {
		t.Fatalf("args = %v", db.args)
	}
	args := db.args[0]
	if args[0] != testEventID || args[1] != "notification-1" || args[2] != "application.decision" ||
		args[4] != now || args[5] != "auth0|private-user-id" {
		t.Errorf("args = %v", args)
	}
	raw, ok := args[3].(json.RawMessage)
	if !ok {
		t.Fatalf("payload arg is %T", args[3])
	}
	var p Payload
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	if p.EventID != testEventID || p.Application.AuthoritySlug == nil || *p.Application.AuthoritySlug != "leeds" {
		t.Errorf("payload = %s", raw)
	}
}

func TestOutbox_Enqueue_Errors(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	db := &fakeExecer{err: boom}
	outbox := NewOutbox(db, fakeSlugs{}, func() string { return testEventID }, time.Now)

	err := outbox.Enqueue(t.Context(), testNotification(notifications.EventNewApplication, nil, "Zone"), testApplication(), "")
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want it to wrap the database error", err)
	}

	db.args = nil
	if err := outbox.Enqueue(t.Context(), testNotification("Other", nil, "Zone"), testApplication(), ""); err == nil || db.args != nil {
		t.Errorf("unknown event type: err = %v, queries = %d", err, len(db.args))
	}
}

var _ querier = (*failingQuerier)(nil)

type failingQuerier struct{ err error }

func (f *failingQuerier) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, f.err
}

func (f *failingQuerier) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, f.err
}

func TestPostgresStore_WrapsDatabaseErrors(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	s := NewPostgresStore(&failingQuerier{err: boom})
	ctx := t.Context()
	ids := []string{"d1"}
	now := time.Now()

	calls := map[string]func() error{
		"ByUserID":                   func() error { _, err := s.ByUserID(ctx, "u"); return err },
		"Create":                     func() error { return s.Create(ctx, Endpoint{}) },
		"Replace":                    func() error { _, err := s.Replace(ctx, "u", "https://example.com", FormatJSON, now); return err },
		"RotateSecret":               func() error { return s.RotateSecret(ctx, "u", "s", now) },
		"ClaimTest":                  func() error { _, err := s.ClaimTest(ctx, "u", now); return err },
		"DeleteAllByUserID":          func() error { return s.DeleteAllByUserID(ctx, "u") },
		"ActiveEndpointsWithPending": func() error { _, err := s.ActiveEndpointsWithPending(ctx); return err },
		"PendingDeliveries":          func() error { _, err := s.PendingDeliveries(ctx, "e", 1); return err },
		"MarkDelivered":              func() error { return s.MarkDelivered(ctx, ids, now) },
		"RecordFailedAttempt":        func() error { _, err := s.RecordFailedAttempt(ctx, ids, now); return err },
		"RecordEndpointSuccess":      func() error { return s.RecordEndpointSuccess(ctx, "e", now) },
		"RecordEndpointFailure":      func() error { _, err := s.RecordEndpointFailure(ctx, "e", Result{}, now); return err },
		"DisableEndpoint":            func() error { _, err := s.DisableEndpoint(ctx, "e", DisabledGone, now); return err },
		"PurgeDeliveriesOlderThan":   func() error { _, err := s.PurgeDeliveriesOlderThan(ctx, now); return err },
	}
	for name, call := range calls {
		if err := call(); !errors.Is(err, boom) {
			t.Errorf("%s: err = %v, want it to wrap the database error", name, err)
		}
	}
}
