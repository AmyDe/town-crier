package notifications

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type fakeItems struct {
	result      [][]byte
	err         error
	lastPK      string
	lastQuery   string
	lastParams  map[string]any
	queryCalled bool
}

func (f *fakeItems) QueryItems(_ context.Context, partitionKey, query string, params map[string]any) ([][]byte, error) {
	f.queryCalled = true
	f.lastPK = partitionKey
	f.lastQuery = query
	f.lastParams = params
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

// docJSON builds a Notifications-container document body with the camelCase keys
// the store reads.
func docJSON(t *testing.T, uid, eventType, decision, createdAt string) []byte {
	t.Helper()
	m := map[string]any{
		"applicationUid": uid,
		"eventType":      eventType,
		"createdAt":      createdAt,
	}
	if decision != "" {
		m["decision"] = decision
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal doc: %v", err)
	}
	return b
}

func TestGetLatestUnreadByApplications_ReducesToLatestPerUID(t *testing.T) {
	t.Parallel()
	// Rows arrive newest-first (the query's ORDER BY createdAt DESC). uid-A has
	// two rows; the newer one must win. uid-B has one decision-update row.
	items := &fakeItems{result: [][]byte{
		docJSON(t, "uid-A", "DecisionUpdate", "Permitted", "2026-02-01T10:00:00+00:00"),
		docJSON(t, "uid-A", "NewApplication", "", "2026-01-01T09:00:00+00:00"),
		docJSON(t, "uid-B", "NewApplication", "", "2026-01-15T08:00:00+00:00"),
	}}
	store := NewCosmosStore(items)

	got, err := store.GetLatestUnreadByApplications(
		context.Background(), "user-1", []string{"uid-A", "uid-B"}, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("GetLatestUnreadByApplications: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("map size: got %d, want 2", len(got))
	}
	a := got["uid-A"]
	if a.EventType != EventDecisionUpdate || a.Decision == nil || *a.Decision != "Permitted" {
		t.Errorf("uid-A latest: got %+v", a)
	}
	wantA := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)
	if !a.CreatedAt.Equal(wantA) {
		t.Errorf("uid-A createdAt: got %v, want %v", a.CreatedAt, wantA)
	}
	b := got["uid-B"]
	if b.EventType != EventNewApplication || b.Decision != nil {
		t.Errorf("uid-B latest: got %+v", b)
	}
	if items.lastPK != "user-1" {
		t.Errorf("partition key: got %q, want \"user-1\"", items.lastPK)
	}
	if !strings.Contains(items.lastQuery, "ARRAY_CONTAINS(@uids, c.applicationUid)") {
		t.Errorf("query missing ARRAY_CONTAINS clause: %q", items.lastQuery)
	}
	if got, ok := items.lastParams["@lastReadAt"].(string); !ok || got == "" {
		t.Errorf("@lastReadAt must be the +00:00 DateTimeOffset string form, got %v", items.lastParams["@lastReadAt"])
	}
}

func TestGetLatestUnreadByApplications_EmptyUIDsSkipsQuery(t *testing.T) {
	t.Parallel()
	items := &fakeItems{}
	store := NewCosmosStore(items)

	got, err := store.GetLatestUnreadByApplications(context.Background(), "user-1", nil, time.Now())
	if err != nil {
		t.Fatalf("GetLatestUnreadByApplications: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("empty uids: got %v, want empty non-nil map", got)
	}
	if items.queryCalled {
		t.Error("empty uids must not issue a Cosmos query")
	}
}

func TestGetLatestUnreadByApplications_LegacyNullEventTypeCoalescesToNewApplication(t *testing.T) {
	t.Parallel()
	// A legacy row with no eventType field (predating tc-so3a.3) must hydrate as
	// NewApplication (the legacy backfill coalesce).
	legacy := []byte(`{"applicationUid":"uid-X","createdAt":"2026-01-01T00:00:00+00:00"}`)
	items := &fakeItems{result: [][]byte{legacy}}
	store := NewCosmosStore(items)

	got, err := store.GetLatestUnreadByApplications(
		context.Background(), "user-1", []string{"uid-X"}, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("GetLatestUnreadByApplications: %v", err)
	}
	if got["uid-X"].EventType != EventNewApplication {
		t.Errorf("legacy eventType: got %q, want NewApplication", got["uid-X"].EventType)
	}
}
