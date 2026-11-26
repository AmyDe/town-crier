//go:build integration

package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

// Tests here must not call t.Parallel: pgtest.New serialises them.

var intNow = time.Date(2026, 11, 26, 14, 0, 0, 0, time.UTC)

type fixedSlug string

func (s fixedSlug) SlugForAreaID(int) (string, bool) { return string(s), s != "" }

func newIntStore(t *testing.T) (*PostgresStore, *pgxpool.Pool) {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "webhook_endpoints", "webhook_deliveries")
	return NewPostgresStore(pool), pool
}

func createEndpoint(t *testing.T, s *PostgresStore, userID string) Endpoint {
	t.Helper()
	e, err := NewEndpoint(uuid.NewString(), userID, "https://hooks.example.com/"+userID, FormatJSON, intNow)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Create(context.Background(), e); err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	return e
}

func mustByUserID(t *testing.T, s *PostgresStore, userID string) Endpoint {
	t.Helper()
	e, err := s.ByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("by user id: %v", err)
	}
	return e
}

func enqueueAt(t *testing.T, pool *pgxpool.Pool, userID, notificationID, deliveryID string, at time.Time) {
	t.Helper()
	outbox := NewOutbox(pool, fixedSlug("leeds"), func() string { return deliveryID }, func() time.Time { return at })
	n := testNotification(notifications.EventNewApplication, new(testZoneID), "Zone")
	n.ID, n.UserID = notificationID, userID
	if err := outbox.Enqueue(context.Background(), n, testApplication(), "Home"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
}

type deliveryRow struct {
	Status      string
	Attempts    int
	LastAttempt *time.Time
	DeliveredAt *time.Time
}

func readDelivery(t *testing.T, pool *pgxpool.Pool, id string) deliveryRow {
	t.Helper()
	var r deliveryRow
	err := pool.QueryRow(context.Background(),
		"SELECT status, attempts, last_attempt_at, delivered_at FROM webhook_deliveries WHERE id = $1", id).
		Scan(&r.Status, &r.Attempts, &r.LastAttempt, &r.DeliveredAt)
	if err != nil {
		t.Fatalf("read delivery %s: %v", id, err)
	}
	return r
}

func countDeliveries(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM webhook_deliveries").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func deliveryID(n int) string {
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", n)
}

func TestIntegration_Webhooks_EndpointRoundTrip(t *testing.T) {
	s, _ := newIntStore(t)
	ctx := context.Background()

	if _, err := s.ByUserID(ctx, "u1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing endpoint: err = %v, want ErrNotFound", err)
	}
	want := createEndpoint(t, s, "u1")
	if got := mustByUserID(t, s, "u1"); !reflect.DeepEqual(got, want) {
		t.Errorf("round trip\n got %#v\nwant %#v", got, want)
	}

	second, err := NewEndpoint(uuid.NewString(), "u1", "https://hooks.example.com/again", FormatSlack, intNow)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Create(ctx, second); !errors.Is(err, ErrEndpointExists) {
		t.Errorf("second endpoint: err = %v, want ErrEndpointExists", err)
	}
}

func TestIntegration_Webhooks_ReplaceReactivatesAndKeepsSecret(t *testing.T) {
	s, _ := newIntStore(t)
	ctx := context.Background()
	original := createEndpoint(t, s, "u1")
	if _, err := s.RecordEndpointFailure(ctx, original.ID, original.URL, Result{Failure: FailureGone, Status: 410}, intNow); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DisableEndpoint(ctx, original.ID, original.URL, DisabledGone, intNow); err != nil {
		t.Fatal(err)
	}
	later := intNow.Add(time.Hour)

	got, err := s.Replace(ctx, "u1", "https://new.example.com/hook", FormatSlack, later)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != original.ID || got.Secret != original.Secret || !got.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("identity changed: %#v", got)
	}
	if got.URL != "https://new.example.com/hook" || got.Format != FormatSlack || got.Status != StatusActive ||
		got.DisabledReason != nil || got.ConsecutiveFailures != 0 || !got.UpdatedAt.Equal(later) {
		t.Errorf("not replaced: url %s format %s status %s failures %d", got.URL, got.Format, got.Status, got.ConsecutiveFailures)
	}
	if got.LastFailureAt == nil {
		t.Error("failure history was lost")
	}
	if stored := mustByUserID(t, s, "u1"); !reflect.DeepEqual(stored, got) {
		t.Errorf("returned %#v, stored %#v", got, stored)
	}

	if _, err := s.Replace(ctx, "nobody", "https://new.example.com", FormatJSON, later); !errors.Is(err, ErrNotFound) {
		t.Errorf("replace missing: err = %v", err)
	}
	if _, err := s.Replace(ctx, "u1", "http://new.example.com", FormatJSON, later); !errors.Is(err, ErrInvalidURL) {
		t.Errorf("replace with bad URL: err = %v", err)
	}
}

func TestIntegration_Webhooks_RotateSecret(t *testing.T) {
	s, _ := newIntStore(t)
	ctx := context.Background()
	createEndpoint(t, s, "u1")
	other := createEndpoint(t, s, "u2")
	secret := NewSecret()

	if err := s.RotateSecret(ctx, "u1", secret, intNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got := mustByUserID(t, s, "u1"); got.Secret != secret || !got.UpdatedAt.Equal(intNow.Add(time.Minute)) {
		t.Error("secret was not rotated")
	}
	if got := mustByUserID(t, s, "u2"); got.Secret != other.Secret {
		t.Error("another user's secret changed")
	}
	if err := s.RotateSecret(ctx, "nobody", secret, intNow); !errors.Is(err, ErrNotFound) {
		t.Errorf("rotate missing: err = %v", err)
	}
}

func TestIntegration_Webhooks_ClaimTest(t *testing.T) {
	s, _ := newIntStore(t)
	ctx := context.Background()
	original := createEndpoint(t, s, "u1")

	for _, step := range []struct {
		name  string
		after time.Duration
		want  bool
	}{
		{"first", time.Minute, true},
		{"9s later", time.Minute + 9*time.Second, false},
		{"10s later", time.Minute + 10*time.Second, true},
	} {
		got, err := s.ClaimTest(ctx, "u1", intNow.Add(step.after))
		if err != nil || got != step.want {
			t.Fatalf("%s: ClaimTest = %v, %v, want %v", step.name, got, err, step.want)
		}
	}
	got := mustByUserID(t, s, "u1")
	if got.LastTestAt == nil || !got.LastTestAt.Equal(intNow.Add(time.Minute+10*time.Second)) {
		t.Errorf("lastTestAt = %v", got.LastTestAt)
	}
	got.LastTestAt = nil
	if !reflect.DeepEqual(got, original) {
		t.Errorf("a claim changed more than last_test_at: %#v", got)
	}
	if ok, err := s.ClaimTest(ctx, "nobody", intNow); err != nil || ok {
		t.Errorf("claim missing = %v, %v", ok, err)
	}
}

func TestIntegration_Webhooks_DeleteAllByUserIDCascades(t *testing.T) {
	s, pool := newIntStore(t)
	ctx := context.Background()
	createEndpoint(t, s, "u1")
	createEndpoint(t, s, "u2")
	enqueueAt(t, pool, "u1", "n1", deliveryID(1), intNow)
	enqueueAt(t, pool, "u2", "n2", deliveryID(2), intNow)

	if err := s.DeleteAllByUserID(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ByUserID(ctx, "u1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("endpoint survived: %v", err)
	}
	if got := countDeliveries(t, pool); got != 1 {
		t.Errorf("deliveries = %d, want only the other user's", got)
	}
	mustByUserID(t, s, "u2")
	if err := s.DeleteAllByUserID(ctx, "u1"); err != nil {
		t.Errorf("second delete: %v", err)
	}
}

func TestIntegration_Webhooks_OutboxEnqueue(t *testing.T) {
	s, pool := newIntStore(t)
	ctx := context.Background()

	enqueueAt(t, pool, "u1", "n1", deliveryID(1), intNow)
	if got := countDeliveries(t, pool); got != 0 {
		t.Fatalf("no endpoint: %d rows, want 0", got)
	}

	endpoint := createEndpoint(t, s, "u1")
	createEndpoint(t, s, "u2")
	enqueueAt(t, pool, "u1", "n1", deliveryID(2), intNow)
	if got := countDeliveries(t, pool); got != 1 {
		t.Fatalf("active endpoint: %d rows, want 1", got)
	}
	enqueueAt(t, pool, "u1", "n1", deliveryID(3), intNow.Add(time.Second))
	if got := countDeliveries(t, pool); got != 1 {
		t.Fatalf("same notification twice: %d rows, want 1", got)
	}

	pending, err := s.PendingDeliveries(ctx, endpoint.ID, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("pending = %d", len(pending))
	}
	d := pending[0]
	if d.ID != deliveryID(2) || d.EndpointID != endpoint.ID || d.NotificationID != "n1" ||
		d.EventType != EventApplicationNew || d.Attempts != 0 || !d.CreatedAt.Equal(intNow) {
		t.Errorf("delivery = %+v", d)
	}
	var p Payload
	if err := json.Unmarshal(d.Payload, &p); err != nil {
		t.Fatal(err)
	}
	if p.EventID != d.ID || p.Application.ShareURL == nil || p.WatchZone == nil || p.WatchZone.Name != "Home" ||
		p.Application.StartDate == nil || p.Application.StartDate.String() != "2026-11-20" {
		t.Errorf("payload = %s", d.Payload)
	}
	again, err := s.PendingDeliveries(ctx, endpoint.ID, 100)
	if err != nil {
		t.Fatal(err)
	}
	if string(again[0].Payload) != string(d.Payload) {
		t.Error("the stored payload does not read back byte for byte")
	}

	if _, err := s.DisableEndpoint(ctx, endpoint.ID, DisabledFailures, intNow); err != nil {
		t.Fatal(err)
	}
	enqueueAt(t, pool, "u1", "n2", deliveryID(4), intNow)
	if got := countDeliveries(t, pool); got != 1 {
		t.Errorf("disabled endpoint: %d rows, want 1", got)
	}
}

func TestIntegration_Webhooks_PendingOrderAndActiveEndpoints(t *testing.T) {
	s, pool := newIntStore(t)
	ctx := context.Background()
	busy := createEndpoint(t, s, "busy")
	createEndpoint(t, s, "idle")
	disabled := createEndpoint(t, s, "disabled")
	done := createEndpoint(t, s, "done")

	enqueueAt(t, pool, "busy", "n-late", deliveryID(1), intNow.Add(time.Hour))
	enqueueAt(t, pool, "busy", "n-tie-b", deliveryID(4), intNow)
	enqueueAt(t, pool, "busy", "n-tie-a", deliveryID(3), intNow)
	enqueueAt(t, pool, "busy", "n-early", deliveryID(5), intNow.Add(-time.Hour))
	enqueueAt(t, pool, "disabled", "n-disabled", deliveryID(6), intNow)
	enqueueAt(t, pool, "done", "n-done", deliveryID(7), intNow)
	if _, err := pool.Exec(ctx, "UPDATE webhook_endpoints SET status = 'disabled' WHERE id = $1", disabled.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkDelivered(ctx, []string{deliveryID(7)}, intNow); err != nil {
		t.Fatal(err)
	}

	active, err := s.ActiveEndpointsWithPending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || active[0].ID != busy.ID || active[0].URL != busy.URL || active[0].Secret != busy.Secret {
		t.Errorf("active = %v, want only %s (not idle, disabled or %s)", active, busy.ID, done.ID)
	}

	ids := func(limit int) []string {
		pending, err := s.PendingDeliveries(ctx, busy.ID, limit)
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, len(pending))
		for i, d := range pending {
			out[i] = d.ID
		}
		return out
	}
	if got, want := ids(100), []string{deliveryID(5), deliveryID(3), deliveryID(4), deliveryID(1)}; !slices.Equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
	if got, want := ids(2), []string{deliveryID(5), deliveryID(3)}; !slices.Equal(got, want) {
		t.Errorf("limit = %v, want %v", got, want)
	}
}

func TestIntegration_Webhooks_DeliveryOutcomes(t *testing.T) {
	s, pool := newIntStore(t)
	ctx := context.Background()
	createEndpoint(t, s, "u1")
	enqueueAt(t, pool, "u1", "n1", deliveryID(1), intNow)
	enqueueAt(t, pool, "u1", "n2", deliveryID(2), intNow)
	enqueueAt(t, pool, "u1", "n3", deliveryID(3), intNow)
	at := intNow.Add(time.Hour)

	if err := s.MarkDelivered(ctx, []string{deliveryID(1)}, at); err != nil {
		t.Fatal(err)
	}
	got := readDelivery(t, pool, deliveryID(1))
	if got.Status != "delivered" || got.Attempts != 1 || got.DeliveredAt == nil || !got.DeliveredAt.Equal(at) || got.LastAttempt == nil {
		t.Errorf("delivered row = %+v", got)
	}

	for attempt := 1; attempt <= MaxDeliveryAttempts; attempt++ {
		failed, err := s.RecordFailedAttempt(ctx, []string{deliveryID(2)}, at)
		if err != nil {
			t.Fatal(err)
		}
		wantStatus, wantFailed := "pending", 0
		if attempt == 6 {
			wantStatus, wantFailed = "failed", 1
		}
		row := readDelivery(t, pool, deliveryID(2))
		if row.Status != wantStatus || row.Attempts != attempt || failed != wantFailed || row.LastAttempt == nil || row.DeliveredAt != nil {
			t.Fatalf("attempt %d: row %+v, failed %d", attempt, row, failed)
		}
	}

	if _, err := pool.Exec(ctx, "UPDATE webhook_deliveries SET attempts = 5 WHERE id = $1", deliveryID(3)); err != nil {
		t.Fatal(err)
	}
	failed, err := s.RecordFailedAttempt(ctx, []string{deliveryID(1), deliveryID(2), deliveryID(3)}, at.Add(time.Hour))
	if err != nil || failed != 1 {
		t.Fatalf("batch: failed = %d, %v", failed, err)
	}
	if row := readDelivery(t, pool, deliveryID(1)); row.Status != "delivered" || row.Attempts != 1 {
		t.Errorf("a delivered row changed: %+v", row)
	}
	if row := readDelivery(t, pool, deliveryID(2)); row.Attempts != 6 {
		t.Errorf("a failed row changed: %+v", row)
	}
	if err := s.MarkDelivered(ctx, []string{deliveryID(3)}, at); err != nil {
		t.Fatal(err)
	}
	if row := readDelivery(t, pool, deliveryID(3)); row.Status != "failed" {
		t.Errorf("a failed row was delivered: %+v", row)
	}
	if err := s.MarkDelivered(ctx, nil, at); err != nil {
		t.Errorf("empty MarkDelivered: %v", err)
	}
	if n, err := s.RecordFailedAttempt(ctx, nil, at); err != nil || n != 0 {
		t.Errorf("empty RecordFailedAttempt: %d, %v", n, err)
	}
}

func TestIntegration_Webhooks_EndpointBookkeeping(t *testing.T) {
	s, _ := newIntStore(t)
	ctx := context.Background()
	e := createEndpoint(t, s, "u1")

	for want := 1; want <= 2; want++ {
		got, err := s.RecordEndpointFailure(ctx, e.ID, e.URL, Result{Failure: FailureHTTPStatus, Status: 503}, intNow.Add(time.Duration(want)*time.Hour))
		if err != nil || got != want {
			t.Fatalf("failure %d: count = %d, %v", want, got, err)
		}
	}
	got := mustByUserID(t, s, "u1")
	if got.ConsecutiveFailures != 2 || got.LastFailureAt == nil || !got.LastFailureAt.Equal(intNow.Add(2*time.Hour)) ||
		got.LastFailureReason == nil || *got.LastFailureReason != FailureHTTPStatus ||
		got.LastFailureStatus == nil || *got.LastFailureStatus != 503 || got.Status != StatusActive {
		t.Errorf("after failures: %#v", got)
	}

	if _, err := s.RecordEndpointFailure(ctx, e.ID, e.URL, Result{Failure: FailureTimeout}, intNow.Add(3*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got := mustByUserID(t, s, "u1"); got.LastFailureStatus != nil {
		t.Errorf("status without a response = %d, want null", *got.LastFailureStatus)
	}

	if err := s.RecordEndpointSuccess(ctx, e.ID, e.URL, intNow.Add(4*time.Hour)); err != nil {
		t.Fatal(err)
	}
	got = mustByUserID(t, s, "u1")
	if got.ConsecutiveFailures != 0 || got.LastSuccessAt == nil || !got.LastSuccessAt.Equal(intNow.Add(4*time.Hour)) || got.LastFailureAt == nil {
		t.Errorf("after success: %#v", got)
	}

	if _, err := s.RecordEndpointFailure(ctx, uuid.NewString(), "https://hooks.example.com/u1", Result{Failure: FailureTLS}, intNow); !errors.Is(err, ErrNotFound) {
		t.Errorf("failure of a missing endpoint: err = %v", err)
	}
}

func TestIntegration_Webhooks_DisableEndpointDropsPending(t *testing.T) {
	s, pool := newIntStore(t)
	ctx := context.Background()
	e := createEndpoint(t, s, "u1")
	createEndpoint(t, s, "u2")
	enqueueAt(t, pool, "u1", "n1", deliveryID(1), intNow)
	enqueueAt(t, pool, "u1", "n2", deliveryID(2), intNow)
	enqueueAt(t, pool, "u1", "n3", deliveryID(3), intNow)
	enqueueAt(t, pool, "u2", "n4", deliveryID(4), intNow)
	if err := s.MarkDelivered(ctx, []string{deliveryID(1)}, intNow); err != nil {
		t.Fatal(err)
	}

	disabled, err := s.DisableEndpoint(ctx, e.ID, e.URL, DisabledFailures, intNow.Add(time.Hour))
	if err != nil || !disabled {
		t.Fatalf("DisableEndpoint = %v, %v", disabled, err)
	}
	got := mustByUserID(t, s, "u1")
	if got.Status != StatusDisabled || got.DisabledReason == nil || *got.DisabledReason != DisabledFailures {
		t.Errorf("endpoint = %#v", got)
	}
	for id, want := range map[string]string{deliveryID(1): "delivered", deliveryID(2): "failed", deliveryID(3): "failed", deliveryID(4): "pending"} {
		if row := readDelivery(t, pool, id); row.Status != want {
			t.Errorf("delivery %s = %s, want %s", id, row.Status, want)
		}
	}

	if again, err := s.DisableEndpoint(ctx, e.ID, e.URL, DisabledGone, intNow); err != nil || again {
		t.Errorf("second disable = %v, %v, want false", again, err)
	}
	if got := mustByUserID(t, s, "u1"); *got.DisabledReason != DisabledFailures {
		t.Errorf("reason changed to %s", *got.DisabledReason)
	}
}

func TestIntegration_Webhooks_ReplacedURLIgnoresOldResults(t *testing.T) {
	s, pool := newIntStore(t)
	ctx := context.Background()
	old := createEndpoint(t, s, "u1")
	enqueueAt(t, pool, "u1", "n1", deliveryID(1), intNow)
	replaced, err := s.Replace(ctx, "u1", "https://new.example.com/hook", FormatJSON, intNow.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	later := intNow.Add(time.Hour)

	if _, err := s.RecordEndpointFailure(ctx, old.ID, old.URL, Result{Failure: FailureGone, Status: 410}, later); !errors.Is(err, ErrNotFound) {
		t.Errorf("failure for the old URL: err = %v, want ErrNotFound", err)
	}
	if err := s.RecordEndpointSuccess(ctx, old.ID, old.URL, later); err != nil {
		t.Errorf("success for the old URL: %v", err)
	}
	if disabled, err := s.DisableEndpoint(ctx, old.ID, old.URL, DisabledGone, later); err != nil || disabled {
		t.Errorf("disable for the old URL = %v, %v, want false", disabled, err)
	}

	if got := mustByUserID(t, s, "u1"); !reflect.DeepEqual(got, replaced) {
		t.Errorf("the replaced endpoint changed: %#v", got)
	}
	if row := readDelivery(t, pool, deliveryID(1)); row.Status != "pending" {
		t.Errorf("pending delivery = %s, want pending", row.Status)
	}
}

func TestIntegration_Webhooks_PurgeDeliveries(t *testing.T) {
	s, pool := newIntStore(t)
	ctx := context.Background()
	createEndpoint(t, s, "u1")
	cutoff := intNow.Add(-7 * 24 * time.Hour)
	enqueueAt(t, pool, "u1", "old-pending", deliveryID(1), cutoff.Add(-time.Second))
	enqueueAt(t, pool, "u1", "old-delivered", deliveryID(2), cutoff.Add(-24*time.Hour))
	enqueueAt(t, pool, "u1", "at-cutoff", deliveryID(3), cutoff)
	enqueueAt(t, pool, "u1", "new", deliveryID(4), intNow)
	if err := s.MarkDelivered(ctx, []string{deliveryID(2)}, intNow); err != nil {
		t.Fatal(err)
	}

	purged, err := s.PurgeDeliveriesOlderThan(ctx, cutoff)
	if err != nil || purged != 2 {
		t.Fatalf("purged = %d, %v, want 2", purged, err)
	}
	if got := countDeliveries(t, pool); got != 2 {
		t.Errorf("rows left = %d, want 2", got)
	}
	readDelivery(t, pool, deliveryID(3))
	readDelivery(t, pool, deliveryID(4))
	mustByUserID(t, s, "u1")
}
