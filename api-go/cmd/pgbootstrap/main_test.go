package main

import (
	"strings"
	"testing"
)

const testMIObjectID = "09dd6994-a3d8-4e83-abbd-c4d82888a700"

func validParams() bootstrapParams {
	return bootstrapParams{
		Role: "towncrier_api",
		DB:   "town_crier_dev",
		OID:  testMIObjectID,
	}
}

func TestBuildBootstrapSQL_GrantsLeastPrivilege(t *testing.T) {
	t.Parallel()
	sql, err := buildBootstrapSQL(validParams())
	if err != nil {
		t.Fatalf("buildBootstrapSQL() error = %v", err)
	}

	wantSubstrings := []string{
		"pgaadauth_create_principal_with_oid",
		"'towncrier_api'",
		"'" + testMIObjectID + "'",
		"'service'",
		"GRANT CONNECT ON DATABASE town_crier_dev TO towncrier_api",
		"GRANT USAGE ON SCHEMA public TO towncrier_api",
		"GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO towncrier_api",
		"GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO towncrier_api",
		"ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO towncrier_api",
		"ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO towncrier_api",
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(sql, want) {
			t.Errorf("generated SQL missing %q\n---\n%s", want, sql)
		}
	}
}

func TestBuildBootstrapSQL_GrantsNoElevatedPrivilege(t *testing.T) {
	t.Parallel()
	sql, err := buildBootstrapSQL(validParams())
	if err != nil {
		t.Fatalf("buildBootstrapSQL() error = %v", err)
	}

	upper := strings.ToUpper(sql)
	forbidden := []string{"SUPERUSER", "CREATEDB", "CREATEROLE", "OWNER", "DROP", "ALTER TABLE", "CREATE TABLE"}
	for _, bad := range forbidden {
		if strings.Contains(upper, bad) {
			t.Errorf("generated SQL must not contain %q (least-privilege only)\n---\n%s", bad, sql)
		}
	}
}

func TestBuildBootstrapSQL_RejectsInvalidIdentifiers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		params bootstrapParams
	}{
		{"role injection", bootstrapParams{Role: "api; DROP DATABASE town_crier_dev", DB: "town_crier_dev", OID: testMIObjectID}},
		{"db injection", bootstrapParams{Role: "towncrier_api", DB: "town_crier_dev; --", OID: testMIObjectID}},
		{"oid not a uuid", bootstrapParams{Role: "towncrier_api", DB: "town_crier_dev", OID: "not-a-uuid"}},
		{"empty role", bootstrapParams{Role: "", DB: "town_crier_dev", OID: testMIObjectID}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := buildBootstrapSQL(tc.params); err == nil {
				t.Fatalf("buildBootstrapSQL(%+v) returned nil error, want rejection", tc.params)
			}
		})
	}
}

func TestBuildBootstrapSQL_GuardsPrincipalCreationForIdempotency(t *testing.T) {
	t.Parallel()
	sql, err := buildBootstrapSQL(validParams())
	if err != nil {
		t.Fatalf("buildBootstrapSQL() error = %v", err)
	}
	// The principal-create must be guarded so a re-run is a no-op rather than an
	// "already exists" failure.
	if !strings.Contains(sql, "pg_roles") {
		t.Errorf("generated SQL must guard creation against pg_roles for idempotency\n---\n%s", sql)
	}
}
