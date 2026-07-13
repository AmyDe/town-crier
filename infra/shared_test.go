package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestBuildAuthorityNamesDatatable_RendersKQLPrefix asserts buildAuthorityNamesDatatable renders
// the real authority dataset (api-go/internal/authorities/resources/authorities.json) into a
// well-formed KQL `let names = datatable(...)[...];` prefix — the per-authority Poll HWM
// dashboard grid (tc-yxrjs) depends on this exact shape to `lookup` authority names onto
// AuthorityID.
func TestBuildAuthorityNamesDatatable_RendersKQLPrefix(t *testing.T) {
	t.Parallel()

	got, err := buildAuthorityNamesDatatable(authorityNamesJSONPath)
	if err != nil {
		t.Fatalf("buildAuthorityNamesDatatable(%q): %v", authorityNamesJSONPath, err)
	}

	const wantPrefix = "let names = datatable(AuthorityID:int, Authority:string)["
	if !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("output does not start with %q", wantPrefix)
	}
	if !strings.HasSuffix(got, "];") {
		t.Fatalf("output does not end with %q", "];")
	}
	if !strings.Contains(got, "384,'Aberdeen'") {
		t.Error("output does not contain the known pair 384,'Aberdeen'")
	}

	// Every ' in the output must be a datatable string-literal delimiter, never a bare quote
	// smuggled in from an authority name (which would break the KQL literal). Verified by an
	// independent count read straight from authorities.json: each record contributes exactly
	// two unescaped ' (open + close) and any ' inside a name would be escaped as \' and so not
	// counted here.
	raw, err := os.ReadFile(authorityNamesJSONPath)
	if err != nil {
		t.Fatalf("read %s: %v", authorityNamesJSONPath, err)
	}
	var records []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &records); err != nil {
		t.Fatalf("parse %s: %v", authorityNamesJSONPath, err)
	}

	unescapedQuotes := 0
	for i := 0; i < len(got); i++ {
		if got[i] != '\'' {
			continue
		}
		if i > 0 && got[i-1] == '\\' {
			continue // escaped \'
		}
		unescapedQuotes++
	}
	if want := 2 * len(records); unescapedQuotes != want {
		t.Errorf("unescaped ' count = %d, want %d (2 per record) — an unescaped-quote hazard exists", unescapedQuotes, want)
	}
}

// TestBuildAuthorityNamesDatatable_MissingFileErrors asserts a read failure aborts loudly
// (returns an error) rather than falling back to an empty names mapping — the pulumi program
// must hard-fail rather than silently deploy a dashboard with unresolved authority IDs.
func TestBuildAuthorityNamesDatatable_MissingFileErrors(t *testing.T) {
	t.Parallel()

	if _, err := buildAuthorityNamesDatatable("does-not-exist.json"); err == nil {
		t.Fatal("expected an error for a missing file, got nil")
	}
}
