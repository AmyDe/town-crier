package offercodes

import (
	"context"
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
	items map[string][]byte
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
