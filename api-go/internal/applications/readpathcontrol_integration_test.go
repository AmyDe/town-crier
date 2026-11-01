//go:build integration

package applications

import (
	"context"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

func TestPostgresReadPathStore_GetSetOverwrite(t *testing.T) {
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "read_path_control")
	s := NewPostgresReadPathStore(pool)
	ctx := context.Background()

	if _, found, err := s.Get(ctx); err != nil || found {
		t.Fatalf("Get() on empty table = found %v, err %v", found, err)
	}

	at := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	first := ReadPathControl{ReadPathSwitches: ReadPathSwitches{ThinSync: true}, Reason: "first", UpdatedAt: at}
	second := ReadPathControl{ReadPathSwitches: ReadPathSwitches{ThinList: true, ThinMap: true}, Reason: "second", UpdatedAt: at.Add(time.Minute)}
	for _, c := range []ReadPathControl{first, second} {
		if err := s.Set(ctx, c); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		got, found, err := s.Get(ctx)
		if err != nil || !found {
			t.Fatalf("Get() found %v, err %v", found, err)
		}
		if got.ReadPathSwitches != c.ReadPathSwitches || got.Reason != c.Reason || !got.UpdatedAt.Equal(c.UpdatedAt) {
			t.Errorf("Get() = %+v, want %+v", got, c)
		}
	}
	if n := countRows(t, pool, "read_path_control"); n != 1 {
		t.Errorf("rows = %d, want 1", n)
	}
}
