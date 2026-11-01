//go:build integration

package applications

import (
	"context"
	"strings"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

func TestApplicationLocationsMigration_CreatesPrimaryKeyAndGistIndexes(t *testing.T) {
	pool := pgtest.New(t)
	rows, err := pool.Query(context.Background(), `SELECT indexname, indexdef FROM pg_indexes WHERE tablename = 'application_locations'`)
	if err != nil {
		t.Fatalf("query pg_indexes: %v", err)
	}
	defer rows.Close()
	defs := map[string]string{}
	for rows.Next() {
		var name, def string
		if err := rows.Scan(&name, &def); err != nil {
			t.Fatalf("scan: %v", err)
		}
		defs[name] = def
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}

	if _, ok := defs["application_locations_pkey"]; !ok {
		t.Errorf("indexes = %v, want application_locations_pkey", defs)
	}
	for _, name := range []string{"application_locations_location", "application_locations_geometry"} {
		def, ok := defs[name]
		if !ok {
			t.Errorf("indexes = %v, want %s", defs, name)
			continue
		}
		if !strings.Contains(def, "USING gist") {
			t.Errorf("%s = %q, want a gist index", name, def)
		}
	}
}
