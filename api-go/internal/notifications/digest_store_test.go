package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeDigestItems is a hand-written double for the digest store's consumer-side
// container interface: single-partition query, cross-partition query, and upsert.
type fakeDigestItems struct {
	queryResult [][]byte
	queryErr    error
	queryPK     string
	queryText   string
	queryParams map[string]any

	crossResult [][]byte
	crossErr    error
	crossText   string

	upsertErr  error
	upsertPK   string
	upsertBody []byte
}

func (f *fakeDigestItems) QueryItems(_ context.Context, pk, query string, params map[string]any) ([][]byte, error) {
	f.queryPK = pk
	f.queryText = query
	f.queryParams = params
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	return f.queryResult, nil
}

func (f *fakeDigestItems) QueryItemsCrossPartition(_ context.Context, query string, _ map[string]any) ([][]byte, error) {
	f.crossText = query
	if f.crossErr != nil {
		return nil, f.crossErr
	}
	return f.crossResult, nil
}

func (f *fakeDigestItems) UpsertItem(_ context.Context, pk string, body []byte) error {
	f.upsertPK = pk
	f.upsertBody = body
	return f.upsertErr
}

func fullDocJSON(t *testing.T, id, userID, uid, watchZoneID, createdAt string, emailSent bool) []byte {
	t.Helper()
	m := map[string]any{
		"id":                     id,
		"userId":                 userID,
		"applicationUid":         uid,
		"applicationName":        uid,
		"applicationAddress":     "addr",
		"applicationDescription": "desc",
		"authorityId":            1,
		"eventType":              "NewApplication",
		"createdAt":              createdAt,
		"emailSent":              emailSent,
	}
	if watchZoneID != "" {
		m["watchZoneId"] = watchZoneID
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func TestDigestStore_ByUserSince(t *testing.T) {
	t.Parallel()
	items := &fakeDigestItems{queryResult: [][]byte{
		fullDocJSON(t, "n-1", "user-1", "uid-A", "zone-1", "2026-02-02T00:00:00+00:00", false),
		fullDocJSON(t, "n-2", "user-1", "uid-B", "", "2026-02-01T00:00:00+00:00", false),
	}}
	store := NewDigestStore(items)
	since := time.Date(2026, 1, 26, 0, 0, 0, 0, time.UTC)

	got, err := store.ByUserSince(context.Background(), "user-1", since)
	if err != nil {
		t.Fatalf("ByUserSince: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("count: got %d, want 2", len(got))
	}
	if items.queryPK != "user-1" {
		t.Errorf("partition key: got %q, want user-1", items.queryPK)
	}
	if !strings.Contains(items.queryText, "c.createdAt >= @since") {
		t.Errorf("query missing since clause: %q", items.queryText)
	}
	if _, ok := items.queryParams["@since"].(string); !ok {
		t.Errorf("@since must be the .NET string form, got %v", items.queryParams["@since"])
	}
}

func TestDigestStore_UnsentEmailsByUser(t *testing.T) {
	t.Parallel()
	items := &fakeDigestItems{queryResult: [][]byte{
		fullDocJSON(t, "n-1", "user-1", "uid-A", "zone-1", "2026-02-01T00:00:00+00:00", false),
	}}
	store := NewDigestStore(items)

	got, err := store.UnsentEmailsByUser(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("UnsentEmailsByUser: %v", err)
	}
	if len(got) != 1 || got[0].ID != "n-1" {
		t.Fatalf("got %+v", got)
	}
	if items.queryPK != "user-1" {
		t.Errorf("partition key: got %q, want user-1", items.queryPK)
	}
	if !strings.Contains(items.queryText, "emailSent") {
		t.Errorf("query missing emailSent filter: %q", items.queryText)
	}
}

func TestDigestStore_UserIDsWithUnsentEmails(t *testing.T) {
	t.Parallel()
	// azcosmos cannot serve a cross-partition DISTINCT (the gateway 400s), so the
	// query is a plain cross-partition projection and the store dedupes the rows
	// client-side (tc-b7cm). The fake returns duplicate user ids out of order; the
	// store must collapse them to one each in first-seen order.
	items := &fakeDigestItems{crossResult: [][]byte{
		[]byte(`"u1"`),
		[]byte(`"u2"`),
		[]byte(`"u1"`),
		[]byte(`"u3"`),
		[]byte(`"u2"`),
	}}
	store := NewDigestStore(items)

	got, err := store.UserIDsWithUnsentEmails(context.Background())
	if err != nil {
		t.Fatalf("UserIDsWithUnsentEmails: %v", err)
	}
	want := []string{"u1", "u2", "u3"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("dedup order mismatch: got %v, want %v", got, want)
		}
	}
	// Guard the regression: a cross-partition DISTINCT 400s at the gateway, so the
	// query the store sends must NOT contain DISTINCT.
	if strings.Contains(items.crossText, "DISTINCT") {
		t.Errorf("cross-partition query must not use DISTINCT (gateway 400): %q", items.crossText)
	}
}

func TestDigestStore_MarkEmailSentUpsertsDocument(t *testing.T) {
	t.Parallel()
	items := &fakeDigestItems{}
	store := NewDigestStore(items)
	n := DigestNotification{
		ID:                     "n-1",
		UserID:                 "user-1",
		ApplicationUID:         "uid-A",
		ApplicationName:        "uid-A",
		ApplicationAddress:     "addr",
		ApplicationDescription: "desc",
		AuthorityID:            1,
		EventType:              EventNewApplication,
		CreatedAt:              time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		EmailSent:              true,
	}

	if err := store.MarkEmailSent(context.Background(), n); err != nil {
		t.Fatalf("MarkEmailSent: %v", err)
	}
	if items.upsertPK != "user-1" {
		t.Errorf("upsert partition key: got %q, want user-1", items.upsertPK)
	}
	var back digestDocument
	if err := json.Unmarshal(items.upsertBody, &back); err != nil {
		t.Fatalf("unmarshal upserted body: %v", err)
	}
	if !back.EmailSent {
		t.Error("upserted document should have emailSent true")
	}
}

func TestDigestStore_Create_UpsertsDigestReadableDocument(t *testing.T) {
	t.Parallel()
	items := &fakeDigestItems{}
	store := NewDigestStore(items)
	zoneID := "zone-1"
	n := DigestNotification{
		ID:                     "n-1",
		UserID:                 "user-1",
		ApplicationUID:         "24/0001",
		ApplicationName:        "24/0001",
		WatchZoneID:            &zoneID,
		ApplicationAddress:     "10 High St",
		ApplicationDescription: "Loft conversion",
		AuthorityID:            99,
		EventType:              EventNewApplication,
		Sources:                "Zone",
		CreatedAt:              time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC),
	}

	if err := store.Create(context.Background(), n); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if items.upsertPK != "user-1" {
		t.Errorf("upsert partition key: got %q, want user-1", items.upsertPK)
	}
	// The written document must be byte-compatible with what the digest reader
	// hydrates (digestDocument): camelCase keys, the 90-day TTL, emailSent=false.
	var back digestDocument
	if err := json.Unmarshal(items.upsertBody, &back); err != nil {
		t.Fatalf("unmarshal upserted body: %v", err)
	}
	if back.ID != "n-1" || back.UserID != "user-1" || back.AuthorityID != 99 {
		t.Errorf("round-trip mismatch: %+v", back)
	}
	if back.EmailSent {
		t.Error("a freshly created notification must be unsent (emailSent=false)")
	}
	if back.TTL != ninetyDaysSeconds {
		t.Errorf("ttl: got %d, want %d", back.TTL, ninetyDaysSeconds)
	}
	// And it must hydrate cleanly through the digest read path.
	got := back.toDigest()
	if got.ApplicationUID != "24/0001" || got.WatchZoneID == nil || *got.WatchZoneID != "zone-1" {
		t.Errorf("digest hydration mismatch: %+v", got)
	}
}

func TestDigestStore_GetByUserAndApplication_DedupQuery(t *testing.T) {
	t.Parallel()
	decision := "Permitted"
	doc := map[string]any{
		"id":                     "n-1",
		"userId":                 "user-1",
		"applicationUid":         "24/0001",
		"applicationName":        "24/0001",
		"applicationAddress":     "addr",
		"applicationDescription": "desc",
		"authorityId":            99,
		"eventType":              "DecisionUpdate",
		"decision":               decision,
		"createdAt":              "2026-06-13T08:00:00+00:00",
	}
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	items := &fakeDigestItems{queryResult: [][]byte{body}}
	store := NewDigestStore(items)

	got, err := store.GetByUserAndApplication(context.Background(), "user-1", "24/0001", 99, EventDecisionUpdate)
	if err != nil {
		t.Fatalf("GetByUserAndApplication: %v", err)
	}
	if got == nil || got.ID != "n-1" {
		t.Fatalf("expected the existing notification, got %+v", got)
	}
	if items.queryPK != "user-1" {
		t.Errorf("partition key: got %q, want user-1", items.queryPK)
	}
	// Dedup key is (userId, applicationUid, authorityId, eventType) — authority
	// is part of the key because PlanIt uids collide across councils (tc-th98).
	for _, want := range []string{"c.userId", "c.applicationUid", "c.authorityId", "c.eventType"} {
		if !strings.Contains(items.queryText, want) {
			t.Errorf("dedup query missing %q: %s", want, items.queryText)
		}
	}
	if items.queryParams["@applicationUid"] != "24/0001" || items.queryParams["@authorityId"] != 99 || items.queryParams["@eventType"] != "DecisionUpdate" {
		t.Errorf("params: %v", items.queryParams)
	}
}

func TestDigestStore_GetByUserAndApplication_NoMatchReturnsNil(t *testing.T) {
	t.Parallel()
	store := NewDigestStore(&fakeDigestItems{})
	got, err := store.GetByUserAndApplication(context.Background(), "user-1", "24/0001", 99, EventNewApplication)
	if err != nil {
		t.Fatalf("GetByUserAndApplication: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for no match, got %+v", got)
	}
}

func TestDigestStore_ByUserSinceWrapsError(t *testing.T) {
	t.Parallel()
	items := &fakeDigestItems{queryErr: errors.New("boom")}
	store := NewDigestStore(items)

	_, err := store.ByUserSince(context.Background(), "user-1", time.Now())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "user-1") {
		t.Errorf("error should name the user: %v", err)
	}
}
