//go:build integration

package indexnow

import (
	"context"
	"reflect"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

func TestPostgresStateStore_LoadSaveRoundTrip(t *testing.T) {
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "seo_state")
	store := NewPostgresStateStore(pool)
	ctx := context.Background()

	if _, found, err := store.Load(ctx); err != nil || found {
		t.Fatalf("empty Load = found %v, err %v; want not found", found, err)
	}

	first := map[string]string{"https://towncrierapp.uk/planning": "2026-06-28", "https://towncrierapp.uk/planning/a": ""}
	if err := store.Save(ctx, first); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, found, err := store.Load(ctx)
	if err != nil || !found || !reflect.DeepEqual(got, first) {
		t.Fatalf("Load = %v, found %v, err %v; want %v", got, found, err, first)
	}

	second := map[string]string{"https://towncrierapp.uk/planning": "2026-06-29"}
	if err := store.Save(ctx, second); err != nil {
		t.Fatalf("second Save: %v", err)
	}
	got, _, err = store.Load(ctx)
	if err != nil || !reflect.DeepEqual(got, second) {
		t.Fatalf("Load after overwrite = %v, err %v; want %v", got, err, second)
	}
}
