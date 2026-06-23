package profiles

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// fakeAdminItems is a hand-written adminItems: it stores upserted docs and
// answers cross-partition queries from a fixed result set.
type fakeAdminItems struct {
	upserted     map[string][]byte
	queryResult  [][]byte
	queryErr     error
	pageResult   [][]byte
	pageNext     string
	pageErr      error
	gotQuery     string
	gotParams    map[string]any
	gotPageSize  int
	gotPageToken string
}

func newFakeAdminItems() *fakeAdminItems { return &fakeAdminItems{upserted: map[string][]byte{}} }

func (f *fakeAdminItems) UpsertItem(_ context.Context, partitionKey string, item []byte) error {
	f.upserted[partitionKey] = item
	return nil
}

func (f *fakeAdminItems) QueryItemsCrossPartition(_ context.Context, query string, params map[string]any) ([][]byte, error) {
	f.gotQuery = query
	f.gotParams = params
	return f.queryResult, f.queryErr
}

func (f *fakeAdminItems) QueryPageCrossPartition(_ context.Context, query string, _ map[string]any, pageSize int, continuationToken string) ([][]byte, string, error) {
	f.gotQuery = query
	f.gotPageSize = pageSize
	f.gotPageToken = continuationToken
	return f.pageResult, f.pageNext, f.pageErr
}

func profileDoc(t *testing.T, userID, email string, tier SubscriptionTier) []byte {
	t.Helper()
	p, err := NewProfile(userID, email, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}
	p.Tier = tier
	raw, err := json.Marshal(newProfileDocument(p))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func TestAdminStore_GetByEmail(t *testing.T) {
	t.Parallel()

	items := newFakeAdminItems()
	items.queryResult = [][]byte{profileDoc(t, "auth0|u1", "u@example.com", TierPro)}
	store := NewAdminStore(items)

	got, err := store.GetByEmail(context.Background(), "u@example.com")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if got.UserID != "auth0|u1" || got.Tier != TierPro {
		t.Errorf("got %+v", got)
	}
}

func TestAdminStore_GetByEmail_NotFound(t *testing.T) {
	t.Parallel()

	store := NewAdminStore(newFakeAdminItems()) // empty result set
	if _, err := store.GetByEmail(context.Background(), "missing@example.com"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestAdminStore_GetByOriginalTransactionID(t *testing.T) {
	t.Parallel()

	items := newFakeAdminItems()
	items.queryResult = [][]byte{profileDoc(t, "auth0|u9", "u@example.com", TierPersonal)}
	store := NewAdminStore(items)

	got, err := store.GetByOriginalTransactionID(context.Background(), "orig-99")
	if err != nil {
		t.Fatalf("GetByOriginalTransactionID: %v", err)
	}
	if got.UserID != "auth0|u9" || got.Tier != TierPersonal {
		t.Errorf("got %+v", got)
	}
	if items.gotQuery != "SELECT * FROM c WHERE c.originalTransactionId = @txnId" {
		t.Errorf("query = %q", items.gotQuery)
	}
}

func TestAdminStore_GetByOriginalTransactionID_NotFound(t *testing.T) {
	t.Parallel()

	store := NewAdminStore(newFakeAdminItems()) // empty result set
	if _, err := store.GetByOriginalTransactionID(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestAdminStore_Save(t *testing.T) {
	t.Parallel()

	items := newFakeAdminItems()
	store := NewAdminStore(items)
	p, _ := NewProfile("auth0|u1", "u@example.com", time.Now())

	if err := store.Save(context.Background(), p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, ok := items.upserted["auth0|u1"]; !ok {
		t.Error("profile not upserted under its user id")
	}
}

func TestAdminStore_List_FiltersBySearch(t *testing.T) {
	t.Parallel()

	items := newFakeAdminItems()
	items.pageResult = [][]byte{
		profileDoc(t, "auth0|u1", "alice@example.com", TierFree),
		profileDoc(t, "auth0|u2", "bob@example.com", TierPro),
	}
	items.pageNext = "next-token"
	store := NewAdminStore(items)

	page, err := store.List(context.Background(), "example", 20, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(page.Profiles) != 2 || page.ContinuationToken != "next-token" {
		t.Errorf("page = %+v", page)
	}
	// A non-empty search uses the CONTAINS filter and forwards the page size.
	if items.gotQuery != "SELECT * FROM c WHERE CONTAINS(c.email, @search, true)" {
		t.Errorf("query = %q", items.gotQuery)
	}
	if items.gotPageSize != 20 {
		t.Errorf("pageSize forwarded = %d, want 20", items.gotPageSize)
	}
}

func TestAdminStore_List_NoSearchScansAll(t *testing.T) {
	t.Parallel()

	items := newFakeAdminItems()
	store := NewAdminStore(items)

	if _, err := store.List(context.Background(), "", 20, "tok"); err != nil {
		t.Fatalf("List: %v", err)
	}
	if items.gotQuery != "SELECT * FROM c" {
		t.Errorf("query = %q, want unfiltered scan", items.gotQuery)
	}
	if items.gotPageToken != "tok" {
		t.Errorf("continuation token forwarded = %q, want tok", items.gotPageToken)
	}
}

func TestUserProfile_ExpireSubscription(t *testing.T) {
	t.Parallel()

	p, _ := NewProfile("auth0|u1", "u@example.com", time.Now())
	p.ActivateSubscription(TierPro, time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC))

	p.ExpireSubscription()

	if p.Tier != TierFree || p.SubscriptionExpiry != nil || p.GracePeriodExpiry != nil {
		t.Errorf("after expire: tier=%v expiry=%v grace=%v", p.Tier, p.SubscriptionExpiry, p.GracePeriodExpiry)
	}
}

func TestAdminStore_ByDigestDay(t *testing.T) {
	t.Parallel()

	items := newFakeAdminItems()
	items.queryResult = [][]byte{
		profileDoc(t, "auth0|u1", "u1@example.com", TierPro),
		profileDoc(t, "auth0|u2", "u2@example.com", TierFree),
	}
	store := NewAdminStore(items)

	got, err := store.ByDigestDay(context.Background(), time.Wednesday)
	if err != nil {
		t.Fatalf("ByDigestDay: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("count: got %d, want 2", len(got))
	}
	if items.gotQuery != "SELECT * FROM c WHERE c.digestDay = @digestDay" {
		t.Errorf("query = %q", items.gotQuery)
	}
	// The digest day is bound as the int weekday value (Sunday=0 … Saturday=6);
	// Wednesday must bind as 3.
	if day, ok := items.gotParams["@digestDay"].(int); !ok || day != int(time.Wednesday) {
		t.Errorf("@digestDay = %v, want int %d", items.gotParams["@digestDay"], int(time.Wednesday))
	}
}

func TestAdminStore_ByDigestDayWrapsError(t *testing.T) {
	t.Parallel()

	items := newFakeAdminItems()
	items.queryErr = errors.New("boom")
	store := NewAdminStore(items)

	if _, err := store.ByDigestDay(context.Background(), time.Monday); err == nil {
		t.Fatal("expected error")
	}
}

// profileDocAt builds a stored profile document with a specific LastActiveAt so
// the dormant scan's Go-side cutoff comparison can be exercised.
func profileDocAt(t *testing.T, userID string, lastActive time.Time) []byte {
	t.Helper()
	p, err := NewProfile(userID, "", lastActive)
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}
	p.LastActiveAt = lastActive
	raw, err := json.Marshal(newProfileDocument(p))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func TestAdminStore_Dormant_KeepsAccountsActiveAtOrAfterCutoff(t *testing.T) {
	t.Parallel()
	cutoff := time.Date(2025, 6, 14, 0, 0, 0, 0, time.UTC)

	items := newFakeAdminItems()
	items.queryResult = [][]byte{
		profileDocAt(t, "dormant-13mo", cutoff.AddDate(0, -1, 0)), // before cutoff -> dormant
		profileDocAt(t, "active-11mo", cutoff.AddDate(0, 1, 0)),   // after cutoff -> kept
		profileDocAt(t, "exactly-12mo", cutoff),                   // == cutoff -> kept (not strictly before)
		profileDocAt(t, "dormant-old", cutoff.AddDate(-1, 0, 0)),  // well before -> dormant
	}
	store := NewAdminStore(items)

	got, err := store.Dormant(context.Background(), cutoff)
	if err != nil {
		t.Fatalf("Dormant: %v", err)
	}

	ids := map[string]bool{}
	for _, p := range got {
		ids[p.UserID] = true
	}
	if !ids["dormant-13mo"] || !ids["dormant-old"] {
		t.Errorf("dormant accounts missing from result: %v", ids)
	}
	if ids["active-11mo"] {
		t.Error("an account active after the cutoff must not be dormant")
	}
	if ids["exactly-12mo"] {
		t.Error("an account active exactly at the cutoff must be kept (strictly-before semantics)")
	}
	if len(got) != 2 {
		t.Errorf("dormant count: got %d, want 2", len(got))
	}
}

// legacyProfileDocAt builds a stored profile document WITHOUT the
// lastActiveAtEpoch field, simulating an un-backfilled document written before
// the numeric mirror existed. Such docs are returned by the server-side
// NOT IS_DEFINED fallback and must still be classified correctly by the Go gate.
func legacyProfileDocAt(t *testing.T, userID string, lastActive time.Time) []byte {
	t.Helper()
	p, err := NewProfile(userID, "", lastActive)
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}
	p.LastActiveAt = lastActive
	var doc map[string]any
	if err := json.Unmarshal(profileDocAt(t, userID, lastActive), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	delete(doc, "lastActiveAtEpoch")
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func TestAdminStore_Dormant_FiltersServerSide(t *testing.T) {
	t.Parallel()
	cutoff := time.Date(2025, 6, 14, 0, 0, 0, 0, time.UTC)

	items := newFakeAdminItems()
	store := NewAdminStore(items)

	if _, err := store.Dormant(context.Background(), cutoff); err != nil {
		t.Fatalf("Dormant: %v", err)
	}

	// The scan filters server-side on the numeric epoch mirror, shrinking the
	// hydrated set from "all users" to "dormant users" — while the NOT IS_DEFINED
	// fallback keeps any un-backfilled (legacy) doc in the result set so it is
	// never silently dropped during rollout.
	const want = "SELECT * FROM c WHERE c.lastActiveAtEpoch < @cutoffEpoch OR NOT IS_DEFINED(c.lastActiveAtEpoch)"
	if items.gotQuery != want {
		t.Errorf("query = %q, want %q", items.gotQuery, want)
	}
	// The cutoff binds as epoch milliseconds so it sorts against the numeric field.
	if epoch, ok := items.gotParams["@cutoffEpoch"].(int64); !ok || epoch != cutoff.UnixMilli() {
		t.Errorf("@cutoffEpoch = %v, want int64 %d", items.gotParams["@cutoffEpoch"], cutoff.UnixMilli())
	}
}

func TestAdminStore_Dormant_ClassifiesLegacyDocsInGo(t *testing.T) {
	t.Parallel()
	cutoff := time.Date(2025, 6, 14, 0, 0, 0, 0, time.UTC)

	// Legacy docs lack lastActiveAtEpoch, so the server returns them via the
	// NOT IS_DEFINED fallback regardless of activity. The Go-side cutoff gate is
	// the final authority: a legacy-but-active account must NOT be erased, and a
	// legacy-and-dormant account must still be caught.
	items := newFakeAdminItems()
	items.queryResult = [][]byte{
		legacyProfileDocAt(t, "legacy-dormant", cutoff.AddDate(0, -1, 0)), // before cutoff -> dormant
		legacyProfileDocAt(t, "legacy-active", cutoff.AddDate(0, 1, 0)),   // after cutoff -> kept
		profileDocAt(t, "backfilled-dormant", cutoff.AddDate(-1, 0, 0)),   // backfilled, dormant
	}
	store := NewAdminStore(items)

	got, err := store.Dormant(context.Background(), cutoff)
	if err != nil {
		t.Fatalf("Dormant: %v", err)
	}

	ids := map[string]bool{}
	for _, p := range got {
		ids[p.UserID] = true
	}
	if !ids["legacy-dormant"] || !ids["backfilled-dormant"] {
		t.Errorf("dormant accounts missing from result: %v", ids)
	}
	if ids["legacy-active"] {
		t.Error("a legacy doc active after the cutoff must not be erased (Go gate is the final authority)")
	}
	if len(got) != 2 {
		t.Errorf("dormant count: got %d, want 2", len(got))
	}
}

func TestAdminStore_Dormant_WrapsQueryError(t *testing.T) {
	t.Parallel()
	items := newFakeAdminItems()
	items.queryErr = errors.New("cosmos down")
	store := NewAdminStore(items)

	if _, err := store.Dormant(context.Background(), time.Now()); err == nil {
		t.Fatal("expected error when the dormant scan fails")
	}
}

// paidProfileDoc builds a stored profile document with a specific tier, expiry,
// and grace period so the lapsed-paid scan's Go-side EffectiveTier filter can be
// exercised across the window/grace/far-future combinations.
func paidProfileDoc(t *testing.T, userID string, tier SubscriptionTier, expiry, grace *time.Time) []byte {
	t.Helper()
	p, err := NewProfile(userID, "", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}
	p.Tier = tier
	p.SubscriptionExpiry = expiry
	p.GracePeriodExpiry = grace
	raw, err := json.Marshal(newProfileDocument(p))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func TestAdminStore_LapsedPaid_SelectsExpiredPaidProfiles(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	past := now.AddDate(0, 0, -1)
	future := now.AddDate(0, 0, 1)
	farFuture := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)

	items := newFakeAdminItems()
	items.queryResult = [][]byte{
		paidProfileDoc(t, "lapsed-personal", TierPersonal, &past, nil),        // selected: expired, no grace
		paidProfileDoc(t, "lapsed-pro", TierPro, &past, nil),                  // selected: expired, no grace
		paidProfileDoc(t, "expired-grace-past", TierPersonal, &past, &past),   // selected: expired, grace also past
		paidProfileDoc(t, "within-window", TierPro, &future, nil),             // kept: still within window
		paidProfileDoc(t, "expired-grace-live", TierPersonal, &past, &future), // kept: live grace period
		paidProfileDoc(t, "far-future-pro", TierPro, &farFuture, nil),         // kept: 2099 pro-domain/admin grant
		paidProfileDoc(t, "nil-expiry-paid", TierPro, nil, nil),               // kept: malformed paid, treated as entitled
		profileDoc(t, "free-user", "f@example.com", TierFree),                 // kept: free is never paid
	}
	store := NewAdminStore(items)

	got, err := store.LapsedPaid(context.Background(), now)
	if err != nil {
		t.Fatalf("LapsedPaid: %v", err)
	}

	selected := map[string]bool{}
	for _, p := range got {
		selected[p.UserID] = true
	}
	for _, id := range []string{"lapsed-personal", "lapsed-pro", "expired-grace-past"} {
		if !selected[id] {
			t.Errorf("%s should be selected as lapsed-paid", id)
		}
	}
	for _, id := range []string{"within-window", "expired-grace-live", "far-future-pro", "nil-expiry-paid", "free-user"} {
		if selected[id] {
			t.Errorf("%s must NOT be selected (still entitled or free)", id)
		}
	}
	if len(got) != 3 {
		t.Errorf("lapsed count: got %d, want 3", len(got))
	}
	// A full cross-partition scan, filtered in Go — mirrors Dormant's RU profile.
	if items.gotQuery != "SELECT * FROM c" {
		t.Errorf("query = %q, want full scan", items.gotQuery)
	}
}

func TestAdminStore_LapsedPaid_WrapsQueryError(t *testing.T) {
	t.Parallel()
	items := newFakeAdminItems()
	items.queryErr = errors.New("cosmos down")
	store := NewAdminStore(items)

	if _, err := store.LapsedPaid(context.Background(), time.Now()); err == nil {
		t.Fatal("expected error when the lapsed-paid scan fails")
	}
}
