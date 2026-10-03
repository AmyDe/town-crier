//go:build integration

package polling

import (
	"context"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

func TestPostgresPollControlStore_GetSetOverwrite(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "poll_control")
	s := NewPostgresPollControlStore(pool)

	if _, found, err := s.Get(ctx); err != nil || found {
		t.Fatalf("empty table: found = %v err = %v", found, err)
	}

	first := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	if err := s.Set(ctx, PollControl{Enabled: true, Reason: "go live", UpdatedAt: first}); err != nil {
		t.Fatal(err)
	}
	second := first.Add(time.Hour)
	if err := s.Set(ctx, PollControl{Enabled: false, Reason: "dev trial", UpdatedAt: second}); err != nil {
		t.Fatal(err)
	}

	got, found, err := s.Get(ctx)
	if err != nil || !found {
		t.Fatalf("found = %v err = %v", found, err)
	}
	if got.Enabled || got.Reason != "dev trial" || !got.UpdatedAt.Equal(second) {
		t.Fatalf("got %+v", got)
	}
	var rows int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM poll_control").Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("rows = %d err = %v", rows, err)
	}
}
