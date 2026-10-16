package subscriptions

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/subscriptionevents"
)

type notifyCall struct {
	before, after subscriptionevents.Snapshot
	source        subscriptionevents.Source
}

type spyNotifier struct{ calls []notifyCall }

func (s *spyNotifier) Changed(_ context.Context, before, after subscriptionevents.Snapshot, source subscriptionevents.Source) {
	s.calls = append(s.calls, notifyCall{before, after, source})
}

func TestProductProAnnual_MatchesEventsPlanRule(t *testing.T) {
	t.Parallel()
	p := &profiles.UserProfile{Tier: profiles.TierPro, SubscriptionProductID: new(ProductProAnnual)}
	if got := subscriptionevents.SnapshotOf(p).Plan; got != subscriptionevents.PlanAnnual {
		t.Errorf("plan = %q, want annual", got)
	}
}

func TestProcessor_NotifiesOnceAfterSave(t *testing.T) {
	t.Parallel()
	d := newTestProcessor(testAllowedEnvs)
	d.byTxn.profile = freshProfile(t)
	d.verifier.results["hdr.OUTER.sig"] = notificationJSON("SUBSCRIBED", "uuid-1", "INNER")
	d.verifier.results["INNER"] = txnJSON(ProductProAnnual, testBundleID, "orig-1", futureExpiryMs())

	if _, err := d.processor.Process(context.Background(), "hdr.OUTER.sig"); err != nil {
		t.Fatal(err)
	}

	if len(d.notifier.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(d.notifier.calls))
	}
	c := d.notifier.calls[0]
	wantBefore := subscriptionevents.Snapshot{Tier: profiles.TierFree, Plan: subscriptionevents.PlanNone}
	wantAfter := subscriptionevents.Snapshot{Tier: profiles.TierPro, Plan: subscriptionevents.PlanAnnual}
	if c.source != subscriptionevents.SourceAppStore || c.before != wantBefore || c.after != wantAfter {
		t.Errorf("call = %+v", c)
	}
}

func TestProcessor_DoesNotNotify(t *testing.T) {
	t.Parallel()
	t.Run("save failure", func(t *testing.T) {
		t.Parallel()
		d := newTestProcessor(testAllowedEnvs)
		d.byTxn.profile = freshProfile(t)
		d.byTxn.saveErr = errors.New("db down")
		d.verifier.results["hdr.OUTER.sig"] = notificationJSON("SUBSCRIBED", "uuid-1", "INNER")
		d.verifier.results["INNER"] = txnJSON(ProductProMonthly, testBundleID, "orig-1", futureExpiryMs())
		if _, err := d.processor.Process(context.Background(), "hdr.OUTER.sig"); err == nil {
			t.Fatal("want error")
		}
		if len(d.notifier.calls) != 0 {
			t.Errorf("calls = %d, want 0", len(d.notifier.calls))
		}
	})
	t.Run("duplicate", func(t *testing.T) {
		t.Parallel()
		d := newTestProcessor(testAllowedEnvs)
		d.byTxn.profile = freshProfile(t)
		d.idempotency.processed["uuid-1"] = true
		d.verifier.results["hdr.OUTER.sig"] = notificationJSON("SUBSCRIBED", "uuid-1", "INNER")
		d.verifier.results["INNER"] = txnJSON(ProductProMonthly, testBundleID, "orig-1", futureExpiryMs())
		if _, err := d.processor.Process(context.Background(), "hdr.OUTER.sig"); err != nil {
			t.Fatal(err)
		}
		if len(d.notifier.calls) != 0 {
			t.Errorf("calls = %d, want 0", len(d.notifier.calls))
		}
	})
	t.Run("no change", func(t *testing.T) {
		t.Parallel()
		d := newTestProcessor(testAllowedEnvs)
		d.byTxn.profile = freshProfile(t)
		d.verifier.results["hdr.OUTER.sig"] = renewalPrefJSON("DOWNGRADE", "uuid-1")
		d.verifier.results["INNER"] = txnJSON(ProductProMonthly, testBundleID, "orig-1", futureExpiryMs())
		if _, err := d.processor.Process(context.Background(), "hdr.OUTER.sig"); err != nil {
			t.Fatal(err)
		}
		if len(d.notifier.calls) != 0 {
			t.Errorf("calls = %d, want 0", len(d.notifier.calls))
		}
	})
}

func TestVerify_NotifiesOnceAfterSave(t *testing.T) {
	t.Parallel()
	d := newTestDeps()
	d.byUser.profile = freshProfile(t)
	d.verifier.results["JWS_PRO"] = txnJSON(ProductProMonthly, testBundleID, "orig-1", futureExpiryMs())

	rec := d.serve(t, "/v1/subscriptions/verify", `{"signedTransaction":"JWS_PRO"}`, true)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if len(d.notifier.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(d.notifier.calls))
	}
	c := d.notifier.calls[0]
	wantBefore := subscriptionevents.Snapshot{Tier: profiles.TierFree, Plan: subscriptionevents.PlanNone}
	wantAfter := subscriptionevents.Snapshot{Tier: profiles.TierPro, Plan: subscriptionevents.PlanMonthly}
	if c.source != subscriptionevents.SourceAppStore || c.before != wantBefore || c.after != wantAfter {
		t.Errorf("call = %+v", c)
	}
}

func TestVerify_DoesNotNotifyWhenSaveFails(t *testing.T) {
	t.Parallel()
	d := newTestDeps()
	d.byUser.profile = freshProfile(t)
	d.byUser.saveErr = errors.New("db down")
	d.verifier.results["JWS_PRO"] = txnJSON(ProductProMonthly, testBundleID, "orig-1", futureExpiryMs())

	rec := d.serve(t, "/v1/subscriptions/verify", `{"signedTransaction":"JWS_PRO"}`, true)

	if rec.Code == http.StatusOK {
		t.Fatal("want failure")
	}
	if len(d.notifier.calls) != 0 {
		t.Errorf("calls = %d, want 0", len(d.notifier.calls))
	}
}
