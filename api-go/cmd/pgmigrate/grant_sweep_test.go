package main

import (
	"strings"
	"testing"
)

func TestBuildGrantSweepSQL_ContainsBothGrants(t *testing.T) {
	t.Parallel()

	sql, err := buildGrantSweepSQL("towncrier_api")
	if err != nil {
		t.Fatalf("buildGrantSweepSQL() error = %v", err)
	}

	wantSubstrings := []string{
		"GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO towncrier_api",
		"GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO towncrier_api",
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(sql, want) {
			t.Errorf("grant sweep SQL missing %q\n---\n%s", want, sql)
		}
	}
}

func TestBuildGrantSweepSQL_TargetsAllTablesAndSequences(t *testing.T) {
	t.Parallel()

	sql, err := buildGrantSweepSQL("towncrier_api")
	if err != nil {
		t.Fatalf("buildGrantSweepSQL() error = %v", err)
	}

	// The sweep must be ownership-agnostic: ON ALL TABLES / ALL SEQUENCES, never
	// an enumerated table list (which would miss whatever the latest migration
	// created — the very bug this closes).
	for _, want := range []string{
		"ON ALL TABLES IN SCHEMA public",
		"ON ALL SEQUENCES IN SCHEMA public",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("grant sweep SQL missing %q\n---\n%s", want, sql)
		}
	}
}

func TestBuildGrantSweepSQL_RejectsInvalidRole(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		role string
	}{
		{"sql injection via semicolon", "towncrier_api; DROP TABLE notifications"},
		{"leading digit", "1abc"},
		{"empty after default resolution", ""},
		{"embedded space", "town crier"},
		{"double quote", `town"crier`},
		{"single quote", "town'crier"},
		{"trailing comment", "towncrier_api --"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sql, err := buildGrantSweepSQL(tc.role)
			if err == nil {
				t.Fatalf("buildGrantSweepSQL(%q) = nil error, want rejection", tc.role)
			}
			if sql != "" {
				t.Errorf("buildGrantSweepSQL(%q) emitted SQL %q on rejection, want empty", tc.role, sql)
			}
		})
	}
}
