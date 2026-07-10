package digest

import (
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/notifications"
)

// decisionNotif returns a DecisionUpdate record for the same application as
// zoneNotif(uid, zoneID), with its own record ID and a UK-displayable
// decision. This mirrors the real duplicate the polling ingester can produce
// when an application is first seen already-decided (tc-txkm1 root cause):
// both a NewApplication and a DecisionUpdate record land for one
// (ApplicationUID, AuthorityID) pair.
func decisionNotif(uid, zoneID, decision string) notifications.DigestNotification {
	n := zoneNotif(uid, zoneID)
	n.ID = "n-" + uid + "-decision"
	n.EventType = notifications.EventDecisionUpdate
	n.Decision = strptr(decision)
	n.CreatedAt = n.CreatedAt.Add(time.Hour) // decision arrives after the new-application record
	return n
}

func TestDedupByApplication_DecisionUpdateWinsOverNewApplication(t *testing.T) {
	t.Parallel()
	newApp := zoneNotif("uid-A", "zone-1")
	decision := decisionNotif("uid-A", "zone-1", "Permitted")

	got := dedupByApplication([]notifications.DigestNotification{newApp, decision})

	if len(got) != 1 {
		t.Fatalf("deduped count: got %d, want 1", len(got))
	}
	if got[0].EventType != notifications.EventDecisionUpdate {
		t.Errorf("winner event type: got %v, want DecisionUpdate", got[0].EventType)
	}
}

func TestDedupByApplication_DistinctApplicationsUnaffected(t *testing.T) {
	t.Parallel()
	a := zoneNotif("uid-A", "zone-1")
	b := zoneNotif("uid-B", "zone-1")

	got := dedupByApplication([]notifications.DigestNotification{a, b})

	if len(got) != 2 {
		t.Fatalf("deduped count: got %d, want 2 (distinct applications)", len(got))
	}
}

func TestDedupByApplication_DifferentAuthorityIsNotADuplicate(t *testing.T) {
	t.Parallel()
	// Identity is (ApplicationUID, AuthorityID) — the same uid under two
	// authorities is a coincidence, not a duplicate (project convention).
	a := zoneNotif("uid-A", "zone-1")
	b := zoneNotif("uid-A", "zone-1")
	b.ID = "n-uid-A-other-authority"
	b.AuthorityID = 2

	got := dedupByApplication([]notifications.DigestNotification{a, b})

	if len(got) != 2 {
		t.Fatalf("different authorities must not collapse: got %d, want 2", len(got))
	}
}

func TestDedupByApplication_ZoneAttributedWinsOverSavedOnlyWhenEventTypesEqual(t *testing.T) {
	t.Parallel()
	savedOnly := zoneNotif("uid-A", "")
	savedOnly.WatchZoneID = nil
	savedOnly.ID = "n-uid-A-saved"
	zoneAttributed := zoneNotif("uid-A", "zone-1")
	zoneAttributed.ID = "n-uid-A-zone"

	got := dedupByApplication([]notifications.DigestNotification{savedOnly, zoneAttributed})

	if len(got) != 1 {
		t.Fatalf("deduped count: got %d, want 1", len(got))
	}
	if got[0].WatchZoneID == nil {
		t.Errorf("winner should be the zone-attributed record, got saved-only: %+v", got[0])
	}
}

func TestDedupByApplication_DecisionUpdateWinsRegardlessOfZoneAttribution(t *testing.T) {
	t.Parallel()
	// A saved-only DecisionUpdate must still beat a zone-attributed
	// NewApplication — event-type preference outranks zone-attribution
	// preference. The winner keeps its own WatchZoneID (nil here), it does not
	// borrow the loser's zone.
	zoneAttributedNew := zoneNotif("uid-A", "zone-1")
	savedOnlyDecision := decisionNotif("uid-A", "", "Permitted")
	savedOnlyDecision.WatchZoneID = nil

	got := dedupByApplication([]notifications.DigestNotification{zoneAttributedNew, savedOnlyDecision})

	if len(got) != 1 {
		t.Fatalf("deduped count: got %d, want 1", len(got))
	}
	if got[0].EventType != notifications.EventDecisionUpdate {
		t.Errorf("winner should be the DecisionUpdate record even though saved-only: got %+v", got[0])
	}
	if got[0].WatchZoneID != nil {
		t.Errorf("winner should keep its own nil WatchZoneID, not borrow the loser's zone: got %v", got[0].WatchZoneID)
	}
}

func TestDedupByApplication_LatestCreatedAtWinsAmongEqualEventTypeAndZoneAttribution(t *testing.T) {
	t.Parallel()
	older := zoneNotif("uid-A", "zone-1")
	newer := zoneNotif("uid-A", "zone-1")
	newer.ID = "n-uid-A-later"
	newer.CreatedAt = older.CreatedAt.Add(time.Hour)

	got := dedupByApplication([]notifications.DigestNotification{older, newer})

	if len(got) != 1 {
		t.Fatalf("deduped count: got %d, want 1", len(got))
	}
	if got[0].ID != newer.ID {
		t.Errorf("winner should be the later record: got ID %q, want %q", got[0].ID, newer.ID)
	}
}

func TestDedupByApplication_PreservesFirstAppearanceOrderForSections(t *testing.T) {
	t.Parallel()
	// groupByZone relies on first-appearance order to build sections in a
	// stable sequence; dedup must not reorder surviving applications.
	first := zoneNotif("uid-A", "zone-1")
	second := zoneNotif("uid-B", "zone-2")
	decisionForFirst := decisionNotif("uid-A", "zone-1", "Permitted")

	got := dedupByApplication([]notifications.DigestNotification{first, second, decisionForFirst})

	if len(got) != 2 {
		t.Fatalf("deduped count: got %d, want 2", len(got))
	}
	if got[0].ApplicationUID != "uid-A" || got[1].ApplicationUID != "uid-B" {
		t.Errorf("order not preserved: got %+v", got)
	}
}

func TestDedupByApplication_EmptyInput(t *testing.T) {
	t.Parallel()
	got := dedupByApplication(nil)
	if len(got) != 0 {
		t.Errorf("deduped count: got %d, want 0", len(got))
	}
}
