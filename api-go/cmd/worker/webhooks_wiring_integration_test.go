//go:build integration

package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
	"github.com/AmyDe/town-crier/api-go/internal/webhooks"
)

const wiringUserID = "auth0|webhook-wiring"

func newWiringDB(t *testing.T) (*pgxpool.Pool, *stores) {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "webhook_endpoints", "webhook_deliveries", "users", "notifications",
		"watch_zones", "saved_applications", "application_event", "leases", "poll_control")
	return pool, newStores(pool, discardLogger())
}

// seedProWithWebhook stores a Pro user whose webhook points at a loopback
// address, which the production client refuses to dial, so no test request
// leaves the machine.
func seedProWithWebhook(t *testing.T, st *stores, lastActive time.Time) {
	t.Helper()
	ctx := context.Background()
	profile, err := profiles.NewProfile(wiringUserID, "", lastActive)
	if err != nil {
		t.Fatal(err)
	}
	profile.Tier = profiles.TierPro
	if err := st.profile.Save(ctx, profile); err != nil {
		t.Fatalf("save profile: %v", err)
	}
	endpoint, err := webhooks.NewEndpoint(uuid.NewString(), wiringUserID, "https://hooks.example.com/x", webhooks.FormatJSON, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	endpoint.URL = "https://127.0.0.1/hook"
	if err := st.webhook.Create(ctx, endpoint); err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
}

// queueAlerts runs both production dispatchers for one application inside the
// user's zone, so each creates one record.
func queueAlerts(t *testing.T, pool *pgxpool.Pool, st *stores) {
	t.Helper()
	ctx := context.Background()
	zone, err := watchzones.NewWatchZone(uuid.NewString(), wiringUserID, "Home", 51.5, -0.1, 500,
		time.Now().Add(-48*time.Hour).UTC(), true, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.zone.Save(ctx, zone); err != nil {
		t.Fatalf("save zone: %v", err)
	}
	lat, lng, state := 51.5, -0.1, "Permitted"
	app := applications.PlanningApplication{
		Name: "24/0001", UID: "24/0001", AreaName: "Leeds", AreaID: 99, Address: "1 High Street",
		Description: "Loft conversion", AppState: &state, Latitude: &lat, Longitude: &lng,
		LastDifferent: time.Now().UTC(),
	}
	decision, enqueuer, _ := buildNotifyFanOut(pollTestConfig(), testRegistry(), pool, st.zone, st, discardLogger())
	if err := enqueuer.Enqueue(ctx, app, zone); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if err := decision.Dispatch(ctx, app); err != nil {
		t.Fatalf("dispatch decision: %v", err)
	}
}

func queryStrings(t *testing.T, pool *pgxpool.Pool, sql string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), sql)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestIntegration_WorkerWiring_BothDispatchersQueueWebhooks(t *testing.T) {
	pool, st := newWiringDB(t)
	seedProWithWebhook(t, st, time.Now())

	queueAlerts(t, pool, st)

	got := queryStrings(t, pool,
		"SELECT event_type || ' ' || (payload->'watchZone'->>'name') FROM webhook_deliveries ORDER BY event_type")
	want := []string{"application.decision Home", "application.new Home"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("queued = %v, want %v", got, want)
	}
}

func TestIntegration_WorkerWiring_PollRunDrainsWebhooks(t *testing.T) {
	pool, st := newWiringDB(t)
	seedProWithWebhook(t, st, time.Now())
	queueAlerts(t, pool, st)
	// The polling switch is off by default here, so the run makes no PlanIt call
	// and goes straight to its end-of-run work.
	poller, err := buildPoller(pollTestConfig(), pool, testRegistry(), st, discardLogger())
	if err != nil {
		t.Fatalf("buildPoller: %v", err)
	}

	if err := poller.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	endpoint, err := st.webhook.ByUserID(context.Background(), wiringUserID)
	if err != nil {
		t.Fatal(err)
	}
	if endpoint.ConsecutiveFailures != 1 || endpoint.LastFailureReason == nil ||
		*endpoint.LastFailureReason != webhooks.FailureBlockedAddress {
		t.Fatalf("failures = %d reason = %v, want one blocked_address run", endpoint.ConsecutiveFailures, endpoint.LastFailureReason)
	}
	attempts := queryStrings(t, pool, "SELECT attempts::text FROM webhook_deliveries ORDER BY attempts DESC")
	if len(attempts) != 2 || attempts[0] != "1" || attempts[1] != "0" {
		t.Fatalf("attempts = %v, want only the oldest delivery tried once", attempts)
	}
}

func TestIntegration_WorkerWiring_PurgeRemovesOldDeliveries(t *testing.T) {
	pool, st := newWiringDB(t)
	seedProWithWebhook(t, st, time.Now())
	queueAlerts(t, pool, st)
	if _, err := pool.Exec(context.Background(),
		"UPDATE webhook_deliveries SET created_at = now() - interval '8 days' WHERE event_type = 'application.new'"); err != nil {
		t.Fatal(err)
	}
	cfg := pollTestConfig()
	cfg.NotificationsRetentionDays, cfg.DeviceRegistrationsRetentionDays = 90, 180

	if _, _, err := buildPurger(cfg, pool, st, discardLogger()).Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := queryStrings(t, pool, "SELECT event_type FROM webhook_deliveries")
	if len(got) != 1 || got[0] != "application.decision" {
		t.Fatalf("remaining = %v, want only the recent delivery", got)
	}
}

func TestIntegration_WorkerWiring_DormantCleanupErasesWebhook(t *testing.T) {
	_, st := newWiringDB(t)
	seedProWithWebhook(t, st, time.Now().AddDate(-2, 0, 0))

	deleted, err := buildDormant(pollTestConfig(), st, discardLogger()).Run(context.Background())

	if err != nil || deleted != 1 {
		t.Fatalf("Run = (%d, %v), want 1 account erased", deleted, err)
	}
	if _, err := st.webhook.ByUserID(context.Background(), wiringUserID); !errors.Is(err, webhooks.ErrNotFound) {
		t.Fatalf("webhook after cleanup: err = %v, want ErrNotFound", err)
	}
}
