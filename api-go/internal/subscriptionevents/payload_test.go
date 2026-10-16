package subscriptionevents

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func TestBuildPayload_NeverContainsIdentifiers(t *testing.T) {
	t.Parallel()
	sentinels := []string{"SENTINEL-USER-ID", "SENTINEL-EMAIL@example.test", "SENTINEL-ORIG-TXN", "SENTINEL-LIFETIME-TXN", "SENTINEL-PRODUCT-ID"}
	p := &profiles.UserProfile{
		UserID:                        sentinels[0],
		Email:                         strp(sentinels[1]),
		Tier:                          profiles.TierPro,
		SubscriptionExpiry:            future(),
		OriginalTransactionID:         strp(sentinels[2]),
		LifetimeOriginalTransactionID: strp(sentinels[3]),
		LifetimeTier:                  profiles.TierPro,
		SubscriptionProductID:         strp(sentinels[4]),
	}
	totals := ComputeTotals(10, []*profiles.UserProfile{p}, totalsNow)
	payload := BuildPayload("evt-1", EventPaidStarted, Snapshot{}, SnapshotOf(p), SourceAppStore, totalsNow, totals)

	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if containsAny(string(b), sentinels...) {
		t.Errorf("payload leaks an identifier: %s", b)
	}
}

func TestBuildPayload_Shape(t *testing.T) {
	t.Parallel()
	before := Snapshot{Tier: profiles.TierFree, Plan: PlanNone}
	after := Snapshot{Tier: profiles.TierPro, Plan: PlanAnnual, InTrial: true}
	payload := BuildPayload("evt-1", EventTrialStarted, before, after, SourceAppStore, totalsNow, ComputeTotals(0, nil, totalsNow))
	b, _ := json.Marshal(payload)

	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["schemaVersion"] != float64(1) || got["eventId"] != "evt-1" {
		t.Errorf("envelope = %v", got)
	}
	ev := got["event"].(map[string]any)
	want := map[string]any{
		"type": "trial_started", "fromTier": "Free", "fromPlan": "none",
		"toTier": "Pro", "toPlan": "annual", "source": "app_store", "occurredAt": "2026-10-16T12:00:00Z",
	}
	for k, v := range want {
		if ev[k] != v {
			t.Errorf("event[%s] = %v, want %v", k, ev[k], v)
		}
	}
	totals := got["totals"].(map[string]any)
	for _, k := range []string{"users", "byTier", "byPlan", "paying", "inTrial", "inGrace", "comped"} {
		if _, ok := totals[k]; !ok {
			t.Errorf("totals missing %q", k)
		}
	}
	byPlan := totals["byPlan"].(map[string]any)["Personal"].(map[string]any)
	for _, k := range []string{"monthly", "annual", "lifetime", "comped"} {
		if _, ok := byPlan[k]; !ok {
			t.Errorf("byPlan.Personal missing %q", k)
		}
	}
}

func TestNewEventID_IsUUIDv4(t *testing.T) {
	t.Parallel()
	a, b := newEventID(), newEventID()
	if a == b || len(a) != 36 || a[14] != '4' {
		t.Errorf("ids = %q, %q", a, b)
	}
}
