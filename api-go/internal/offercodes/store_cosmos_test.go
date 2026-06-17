package offercodes

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// fakeItems is an in-memory CosmosItems so the store's serialisation round-trips
// without a real Cosmos.
type fakeItems struct {
	items    map[string][]byte
	queryErr error
}

func newFakeItems() *fakeItems { return &fakeItems{items: map[string][]byte{}} }

func (f *fakeItems) ReadItem(_ context.Context, _, id string) ([]byte, error) {
	raw, ok := f.items[id]
	if !ok {
		return nil, &azcore.ResponseError{StatusCode: http.StatusNotFound}
	}
	return raw, nil
}

func (f *fakeItems) UpsertItem(_ context.Context, partitionKey string, item []byte) error {
	f.items[partitionKey] = item
	return nil
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
