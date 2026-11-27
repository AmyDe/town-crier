//go:build integration

package applications_test

import (
	"context"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

func TestPostgresStore_GetByAuthorityAndName_CollapsedSlash(t *testing.T) {
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "applications")
	store := applications.NewPostgresStore(pool)
	ctx := context.Background()

	seed := func(name string, areaID int) {
		t.Helper()
		err := store.Upsert(ctx, applications.PlanningApplication{
			Name:          name,
			UID:           "uid-" + name,
			AreaName:      "Testshire",
			AreaID:        areaID,
			Address:       "1 Test Street",
			Description:   "a planning application",
			LastDifferent: time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC),
		})
		if err != nil {
			t.Fatalf("Upsert %q: %v", name, err)
		}
	}
	seed("Bromley/23/00178//CON6", 299)
	seed("Bromley/23/00178/FUL", 299)
	seed("A/1//X", 100)
	seed("A/1///X", 100)
	seed("B/2//Y", 200)

	tests := []struct {
		name      string
		authority string
		lookup    string
		wantName  string
		wantFound bool
	}{
		{"collapsed name found", "299", "Bromley/23/00178/CON6", "Bromley/23/00178//CON6", true},
		{"exact double slash still found", "299", "Bromley/23/00178//CON6", "Bromley/23/00178//CON6", true},
		{"exact single slash found", "299", "Bromley/23/00178/FUL", "Bromley/23/00178/FUL", true},
		{"ambiguous collapsed name not found", "100", "A/1/X", "", false},
		{"no cross-authority match", "299", "B/2/Y", "", false},
		{"plain miss", "299", "Bromley/99/99999/FULL", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found, err := store.GetByAuthorityAndName(ctx, tt.authority, tt.lookup)
			if err != nil {
				t.Fatalf("GetByAuthorityAndName: %v", err)
			}
			if found != tt.wantFound {
				t.Fatalf("found = %v, want %v", found, tt.wantFound)
			}
			if found && got.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tt.wantName)
			}
		})
	}
}
