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
	// .NET binds the digest day as the int DayOfWeek value; Go's time.Weekday
	// numbering matches (Sunday=0 … Saturday=6), so Wednesday must bind as 3.
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
