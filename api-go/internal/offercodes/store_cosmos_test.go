package offercodes

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// fakeItems is an in-memory CosmosItems so the store's serialisation round-trips
// without a real Cosmos.
type fakeItems struct {
	items    map[string][]byte
	etags    map[string]string // simulated etags keyed by item id
	queryErr error
	// replaceConflictOnce, when true, causes the first ReplaceItemWithETag call to
	// return ErrCASPreconditionFailed, simulating a concurrent write winning the race.
	replaceConflictOnce  bool
	replaceConflictFired bool
}

func newFakeItems() *fakeItems {
	return &fakeItems{
		items: map[string][]byte{},
		etags: map[string]string{},
	}
}

func (f *fakeItems) ReadItem(_ context.Context, _, id string) ([]byte, error) {
	raw, ok := f.items[id]
	if !ok {
		return nil, &azcore.ResponseError{StatusCode: http.StatusNotFound}
	}
	return raw, nil
}

func (f *fakeItems) UpsertItem(_ context.Context, partitionKey string, item []byte) error {
	f.items[partitionKey] = item
	f.etags[partitionKey] = "etag-" + partitionKey
	return nil
}

// ReadItemWithETag returns the item body and a synthetic etag.
func (f *fakeItems) ReadItemWithETag(_ context.Context, _, id string) ([]byte, string, bool, error) {
	raw, ok := f.items[id]
	if !ok {
		return nil, "", false, nil
	}
	etag := f.etags[id]
	if etag == "" {
		etag = "etag-" + id
	}
	return raw, etag, true, nil
}

// ReplaceItemWithETag replaces the item if the etag matches. When
// replaceConflictOnce is set and the conflict has not yet been fired, it returns
// ErrCASPreconditionFailed without modifying the store.
func (f *fakeItems) ReplaceItemWithETag(_ context.Context, partitionKey, _ string, item []byte, etag string) (string, error) {
	if f.replaceConflictOnce && !f.replaceConflictFired {
		f.replaceConflictFired = true
		return "", platform.ErrCASPreconditionFailed
	}
	current := f.etags[partitionKey]
	if current != "" && current != etag {
		return "", platform.ErrCASPreconditionFailed
	}
	newETag := "etag-replaced-" + partitionKey
	f.items[partitionKey] = item
	f.etags[partitionKey] = newETag
	return newETag, nil
}

// QueryItemsCrossPartition implements the redeemed-by scan the anonymise path
// needs. It honours queryErr and otherwise returns every stored document whose
// redeemedByUserId equals the @userId parameter, mirroring the real
// "SELECT * FROM c WHERE c.redeemedByUserId = @userId" filter.
func (f *fakeItems) QueryItemsCrossPartition(_ context.Context, _ string, params map[string]any) ([][]byte, error) {
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	want, _ := params["@userId"].(string)
	var out [][]byte
	for _, raw := range f.items {
		var doc offerCodeDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, err
		}
		if doc.RedeemedByUserID != nil && *doc.RedeemedByUserID == want {
			out = append(out, raw)
		}
	}
	return out, nil
}

func TestCosmosStore_SaveGetRoundTrip(t *testing.T) {
	t.Parallel()

	store := NewCosmosStore(newFakeItems())
	created := time.Date(2026, 6, 1, 9, 30, 0, 0, time.UTC)
	code, err := NewOfferCode("ABCDEFGHJKMN", profiles.TierPro, 30, created)
	if err != nil {
		t.Fatalf("NewOfferCode: %v", err)
	}

	if err := store.Save(context.Background(), code); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := store.Get(context.Background(), "ABCDEFGHJKMN")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Code != "ABCDEFGHJKMN" || got.Tier != profiles.TierPro || got.DurationDays != 30 {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	if !got.CreatedAt.Equal(created) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, created)
	}
	if got.IsRedeemed() {
		t.Error("freshly stored code should not be redeemed")
	}
}

func TestCosmosStore_RedeemedFieldsRoundTrip(t *testing.T) {
	t.Parallel()

	store := NewCosmosStore(newFakeItems())
	created := time.Date(2026, 6, 1, 9, 30, 0, 0, time.UTC)
	redeemedAt := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	code, _ := NewOfferCode("ABCDEFGHJKMN", profiles.TierPersonal, 14, created)
	if err := code.Redeem("auth0|u1", redeemedAt); err != nil {
		t.Fatalf("Redeem: %v", err)
	}

	if err := store.Save(context.Background(), code); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := store.Get(context.Background(), "ABCDEFGHJKMN")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.IsRedeemed() || got.RedeemedByUserID == nil || *got.RedeemedByUserID != "auth0|u1" {
		t.Errorf("redeemed-by mismatch: %+v", got)
	}
	if got.RedeemedAt == nil || !got.RedeemedAt.Equal(redeemedAt) {
		t.Errorf("RedeemedAt = %v, want %v", got.RedeemedAt, redeemedAt)
	}
}

func TestCosmosStore_Get_NotFound(t *testing.T) {
	t.Parallel()

	store := NewCosmosStore(newFakeItems())
	if _, err := store.Get(context.Background(), "ZZZZZZZZZZZZ"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get missing err = %v, want ErrNotFound", err)
	}
}

// RedeemedByUserID is the GDPR-export read: every code the user redeemed,
// hydrated to the domain model, reusing the same cross-partition redeemedByUserId
// scan the anonymise path uses (the container is keyed by code, not redeemer). It
// must return only the caller's redemptions, not other users' or unredeemed codes.
func TestCosmosStore_RedeemedByUserID(t *testing.T) {
	t.Parallel()

	items := newFakeItems()
	store := NewCosmosStore(items)
	ctx := context.Background()
	created := time.Date(2026, 6, 1, 9, 30, 0, 0, time.UTC)
	redeemedAt := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)

	mine, _ := NewOfferCode("AAAAAAAAAAAA", profiles.TierPro, 30, created)
	if err := mine.Redeem("auth0|target", redeemedAt); err != nil {
		t.Fatalf("Redeem mine: %v", err)
	}
	theirs, _ := NewOfferCode("BBBBBBBBBBBB", profiles.TierPersonal, 14, created)
	if err := theirs.Redeem("auth0|other", redeemedAt); err != nil {
		t.Fatalf("Redeem theirs: %v", err)
	}
	fresh, _ := NewOfferCode("CCCCCCCCCCCC", profiles.TierPro, 7, created)
	for _, c := range []OfferCode{mine, theirs, fresh} {
		if err := store.Save(ctx, c); err != nil {
			t.Fatalf("Save %s: %v", c.Code, err)
		}
	}

	got, err := store.RedeemedByUserID(ctx, "auth0|target")
	if err != nil {
		t.Fatalf("RedeemedByUserID: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("count: got %d, want 1 (only the target's redemption)", len(got))
	}
	c := got[0]
	if c.Code != "AAAAAAAAAAAA" || c.Tier != profiles.TierPro || c.DurationDays != 30 {
		t.Errorf("hydrated code mismatch: %+v", c)
	}
	if c.RedeemedByUserID == nil || *c.RedeemedByUserID != "auth0|target" {
		t.Errorf("redeemer mismatch: %+v", c)
	}
	if c.RedeemedAt == nil || !c.RedeemedAt.Equal(redeemedAt) {
		t.Errorf("RedeemedAt = %v, want %v", c.RedeemedAt, redeemedAt)
	}
}

// A user who never redeemed a code yields an empty, non-nil slice.
func TestCosmosStore_RedeemedByUserID_NoMatches(t *testing.T) {
	t.Parallel()

	store := NewCosmosStore(newFakeItems())
	got, err := store.RedeemedByUserID(context.Background(), "auth0|never")
	if err != nil {
		t.Fatalf("RedeemedByUserID: %v", err)
	}
	if got == nil {
		t.Error("must return a non-nil empty slice, not nil")
	}
	if len(got) != 0 {
		t.Errorf("count: got %d, want 0", len(got))
	}
}

// AnonymiseRedemptionsByUserID scrubs the PII (redeemedByUserId / redeemedAt)
// from every code the user redeemed while keeping the consumed tombstone, and
// must leave codes redeemed by other users — and never-redeemed codes —
// untouched.
func TestCosmosStore_AnonymiseRedemptionsByUserID(t *testing.T) {
	t.Parallel()

	items := newFakeItems()
	store := NewCosmosStore(items)
	ctx := context.Background()
	created := time.Date(2026, 6, 1, 9, 30, 0, 0, time.UTC)
	redeemedAt := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)

	// Target user's redeemed code.
	mine, _ := NewOfferCode("AAAAAAAAAAAA", profiles.TierPro, 30, created)
	if err := mine.Redeem("auth0|target", redeemedAt); err != nil {
		t.Fatalf("Redeem mine: %v", err)
	}
	// Another user's redeemed code — must be untouched.
	theirs, _ := NewOfferCode("BBBBBBBBBBBB", profiles.TierPersonal, 14, created)
	if err := theirs.Redeem("auth0|other", redeemedAt); err != nil {
		t.Fatalf("Redeem theirs: %v", err)
	}
	// An unredeemed code — must be untouched.
	fresh, _ := NewOfferCode("CCCCCCCCCCCC", profiles.TierPro, 7, created)
	for _, c := range []OfferCode{mine, theirs, fresh} {
		if err := store.Save(ctx, c); err != nil {
			t.Fatalf("Save %s: %v", c.Code, err)
		}
	}

	if err := store.AnonymiseRedemptionsByUserID(ctx, "auth0|target"); err != nil {
		t.Fatalf("AnonymiseRedemptionsByUserID: %v", err)
	}

	// The target's code keeps its redeemed state but loses the back-reference.
	gotMine, err := store.Get(ctx, "AAAAAAAAAAAA")
	if err != nil {
		t.Fatalf("Get mine: %v", err)
	}
	if !gotMine.IsRedeemed() {
		t.Error("anonymised code must remain redeemed so it cannot be re-redeemed")
	}
	if gotMine.RedeemedByUserID != nil {
		t.Errorf("redeemedByUserId must be scrubbed, got %v", *gotMine.RedeemedByUserID)
	}
	if gotMine.RedeemedAt != nil {
		t.Errorf("redeemedAt must be scrubbed, got %v", *gotMine.RedeemedAt)
	}

	// The other user's code is unaffected.
	gotTheirs, err := store.Get(ctx, "BBBBBBBBBBBB")
	if err != nil {
		t.Fatalf("Get theirs: %v", err)
	}
	if gotTheirs.RedeemedByUserID == nil || *gotTheirs.RedeemedByUserID != "auth0|other" {
		t.Errorf("another user's redemption must survive, got %+v", gotTheirs)
	}

	// The never-redeemed code stays unredeemed.
	gotFresh, err := store.Get(ctx, "CCCCCCCCCCCC")
	if err != nil {
		t.Fatalf("Get fresh: %v", err)
	}
	if gotFresh.IsRedeemed() {
		t.Error("an unredeemed code must not become redeemed by anonymisation")
	}
}

// A no-match anonymise (the user never redeemed a code, the common case) is a
// no-op success, not an error.
func TestCosmosStore_AnonymiseRedemptionsByUserID_NoMatches(t *testing.T) {
	t.Parallel()

	store := NewCosmosStore(newFakeItems())
	if err := store.AnonymiseRedemptionsByUserID(context.Background(), "auth0|never"); err != nil {
		t.Errorf("no-match anonymise should be a no-op, got %v", err)
	}
}

// A failing cross-partition query propagates so the erasure cascade aborts
// rather than silently leaving the back-reference in place.
func TestCosmosStore_AnonymiseRedemptionsByUserID_QueryError(t *testing.T) {
	t.Parallel()

	items := newFakeItems()
	items.queryErr = errors.New("cosmos down")
	store := NewCosmosStore(items)

	if err := store.AnonymiseRedemptionsByUserID(context.Background(), "auth0|target"); err == nil {
		t.Error("expected the query error to propagate")
	}
}
