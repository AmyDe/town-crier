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

// --- Phase 1 (admin DB: postgres) ---

func TestBuildPrincipalSQL_ContainsPrincipalCreateAndConnectGrant(t *testing.T) {
	t.Parallel()
	sql, err := buildPrincipalSQL(validParams())
	if err != nil {
		t.Fatalf("buildPrincipalSQL() error = %v", err)
	}

	wantSubstrings := []string{
		"pgaadauth_create_principal_with_oid",
		"'towncrier_api'",
		"'" + testMIObjectID + "'",
		"'service'",
		"pg_roles", // idempotency guard
		"GRANT CONNECT ON DATABASE town_crier_dev TO towncrier_api",
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(sql, want) {
			t.Errorf("Phase 1 SQL missing %q\n---\n%s", want, sql)
		}
	}
}

func TestBuildPrincipalSQL_DoesNotContainSchemaGrants(t *testing.T) {
	t.Parallel()
	sql, err := buildPrincipalSQL(validParams())
	if err != nil {
		t.Fatalf("buildPrincipalSQL() error = %v", err)
	}
	// Schema and sequence grants belong in Phase 2 (the app DB). Phase 1 is
	// principal-create + CONNECT only.
	absent := []string{
		"GRANT USAGE ON SCHEMA",
		"GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES",
		"ALTER DEFAULT PRIVILEGES",
	}
	for _, bad := range absent {
		if strings.Contains(sql, bad) {
			t.Errorf("Phase 1 SQL must not contain %q (schema grants belong in Phase 2)\n---\n%s", bad, sql)
		}
	}
}

func TestBuildPrincipalSQL_GrantsNoElevatedPrivilege(t *testing.T) {
	t.Parallel()
	sql, err := buildPrincipalSQL(validParams())
	if err != nil {
		t.Fatalf("buildPrincipalSQL() error = %v", err)
	}
	// Scan executable SQL only: -- comment lines document withheld privileges and
	// must not trip the guard.
	upper := strings.ToUpper(stripSQLComments(sql))
	forbidden := []string{"SUPERUSER", "CREATEDB", "CREATEROLE", "OWNER", "DROP", "CREATE EXTENSION"}
	for _, bad := range forbidden {
		if strings.Contains(upper, bad) {
			t.Errorf("Phase 1 SQL must not contain %q (least-privilege only)\n---\n%s", bad, sql)
		}
	}
}

// --- Phase 2 (app DB: town_crier_dev) ---

func TestBuildGrantsSQL_ContainsDMLGrants(t *testing.T) {
	t.Parallel()
	sql, err := buildGrantsSQL(validParams(), false)
	if err != nil {
		t.Fatalf("buildGrantsSQL() error = %v", err)
	}

	wantSubstrings := []string{
		"GRANT USAGE ON SCHEMA public TO towncrier_api",
		"GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO towncrier_api",
		"GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO towncrier_api",
		"ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO towncrier_api",
		"ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO towncrier_api",
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(sql, want) {
			t.Errorf("Phase 2 SQL missing %q\n---\n%s", want, sql)
		}
	}
}

func TestBuildGrantsSQL_DoesNotContainPrincipalCreate(t *testing.T) {
	t.Parallel()
	sql, err := buildGrantsSQL(validParams(), false)
	if err != nil {
		t.Fatalf("buildGrantsSQL() error = %v", err)
	}
	// Principal create belongs in Phase 1 (the admin DB). Phase 2 is grants only.
	if strings.Contains(sql, "pgaadauth_create_principal_with_oid") {
		t.Errorf("Phase 2 SQL must not contain pgaadauth_create_principal_with_oid (belongs in Phase 1)\n---\n%s", sql)
	}
}

func TestBuildGrantsSQL_GrantsNoElevatedPrivilege(t *testing.T) {
	t.Parallel()
	sql, err := buildGrantsSQL(validParams(), false)
	if err != nil {
		t.Fatalf("buildGrantsSQL() error = %v", err)
	}
	// Scan executable SQL only: -- comment lines document withheld privileges and
	// must not trip the guard.
	upper := strings.ToUpper(stripSQLComments(sql))
	forbidden := []string{"SUPERUSER", "CREATEDB", "CREATEROLE", "OWNER", "DROP", "CREATE EXTENSION"}
	for _, bad := range forbidden {
		if strings.Contains(upper, bad) {
			t.Errorf("Phase 2 SQL must not contain %q (least-privilege only)\n---\n%s", bad, sql)
		}
	}
}

func TestBuildGrantsSQL_ReadonlyTrue_SelectsReadonlyTemplate(t *testing.T) {
	t.Parallel()
	sql, err := buildGrantsSQL(validParams(), true)
	if err != nil {
		t.Fatalf("buildGrantsSQL() error = %v", err)
	}

	wantSubstrings := []string{
		"GRANT USAGE ON SCHEMA public TO towncrier_api",
		"GRANT SELECT ON applications TO towncrier_api",
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(sql, want) {
			t.Errorf("readonly Phase 2 SQL missing %q\n---\n%s", want, sql)
		}
	}

	// Scan executable SQL only: -- comment lines document withheld privileges and
	// must not trip the guard.
	upper := strings.ToUpper(stripSQLComments(sql))
	absent := []string{"INSERT", "UPDATE", "DELETE", "ALTER DEFAULT PRIVILEGES", "SEQUENCE"}
	for _, bad := range absent {
		if strings.Contains(upper, bad) {
			t.Errorf("readonly Phase 2 SQL must not contain %q (read-only role)\n---\n%s", bad, sql)
		}
	}
}

func TestBuildGrantsSQL_ReadonlyFalse_SelectsDefaultDMLTemplate(t *testing.T) {
	t.Parallel()
	sql, err := buildGrantsSQL(validParams(), false)
	if err != nil {
		t.Fatalf("buildGrantsSQL() error = %v", err)
	}
	if !strings.Contains(sql, "GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO towncrier_api") {
		t.Errorf("non-readonly Phase 2 SQL should retain full DML grants\n---\n%s", sql)
	}
}

// --- Shared: identifier/UUID validation ---

func TestBuildPrincipalSQL_RejectsInvalidIdentifiers(t *testing.T) {
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
			if _, err := buildPrincipalSQL(tc.params); err == nil {
				t.Fatalf("buildPrincipalSQL(%+v) returned nil error, want rejection", tc.params)
			}
		})
	}
}

// stripSQLComments removes -- line comments so privilege assertions inspect only
// executable statements.
func stripSQLComments(sql string) string {
	var b strings.Builder
	for _, line := range strings.Split(sql, "\n") {
		if idx := strings.Index(line, "--"); idx >= 0 {
			line = line[:idx]
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}
