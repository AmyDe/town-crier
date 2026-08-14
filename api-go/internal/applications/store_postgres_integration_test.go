//go:build integration

// External test package (applications_test, not applications): watchzones
// imports applications in production code (nearby.go), so a same-package
// test file cannot import watchzones without an import cycle. MatchesExpression
// touches no table, so this test needs no unexported helper from
// store_postgres_test.go — pgtest.New and applications.NewPostgresStore are
// both exported and sufficient.
package applications_test

import (
	"context"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// The two catalog expressions this suite exercises, copied verbatim from
// watchzones.filterCatalog[watchzones.FilterKeyLoftExtension] and
// watchzones.filterCatalog[watchzones.FilterKeyHMOHouseShares]
// (api-go/internal/watchzones/filter.go). filterCatalog itself is
// unexported and the watchzones package has no exported accessor to read a
// single entry's Expression from outside the package, so these cannot be
// pulled in programmatically without adding one to watchzones — out of
// scope for this bead (tc-w825j.4 stays entirely within
// internal/applications/). The IsValidFilterKey checks in the test below at
// least tie this file to the real FilterKey constants, so a renamed or
// removed key fails loudly instead of silently drifting. If watchzones ever
// grows an exported per-key expression accessor, these two constants should
// be deleted in favour of it.
const (
	// loftExtensionExpression mirrors filterCatalog[FilterKeyLoftExtension].Expression.
	loftExtensionExpression = "loft | dormer | (hip <2> gable) | roofspace | (roof <-> space)"
	// hmoHouseSharesExpression mirrors filterCatalog[FilterKeyHMOHouseShares].Expression.
	hmoHouseSharesExpression = "hmo | (multiple <-> occupation) | (class <-> c4) | ((sui <-> generis) & (bedroom | occupation | lodging))"
)

// TestPostgresStore_MatchesExpression proves MatchesExpression evaluates real
// Postgres full-text semantics correctly against a handful of the pre-canned
// watch-zone filter catalog's own validated expressions (GH#1090, epic
// tc-w825j) and real example descriptions.
func TestPostgresStore_MatchesExpression(t *testing.T) {
	if !watchzones.IsValidFilterKey(string(watchzones.FilterKeyLoftExtension)) {
		t.Fatalf("watchzones.FilterKeyLoftExtension is no longer a valid catalog key")
	}
	if !watchzones.IsValidFilterKey(string(watchzones.FilterKeyHMOHouseShares)) {
		t.Fatalf("watchzones.FilterKeyHMOHouseShares is no longer a valid catalog key")
	}

	pool := pgtest.New(t)
	store := applications.NewPostgresStore(pool)
	ctx := context.Background()

	tests := []struct {
		name        string
		description string
		expression  string
		want        bool
	}{
		{
			name:        "loft_extension: description with dormer but not the word loft still matches",
			description: "Erection of a two storey rear extension with a dormer window to the roof",
			expression:  loftExtensionExpression,
			want:        true,
		},
		{
			name:        "loft_extension: hip <2> gable matches the hyphenated Hip-to-gable spelling",
			description: "Single storey side extension with hip-to-gable roof alteration",
			expression:  loftExtensionExpression,
			want:        true,
		},
		{
			name:        "loft_extension: unrelated description does not match",
			description: "Felling of one protected oak tree",
			expression:  loftExtensionExpression,
			want:        false,
		},
		{
			name:        "hmo_house_shares: class <-> c4 excludes a Condition 4 description",
			description: "Discharge of Condition 4 relating to construction hours",
			expression:  hmoHouseSharesExpression,
			want:        false,
		},
		{
			name:        "hmo_house_shares: Class C4 phrase matches",
			description: "Change of use from dwellinghouse to Class C4 HMO",
			expression:  hmoHouseSharesExpression,
			want:        true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := store.MatchesExpression(ctx, tc.description, tc.expression)
			if err != nil {
				t.Fatalf("MatchesExpression: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %v, want %v (description %q, expression %q)", got, tc.want, tc.description, tc.expression)
			}
		})
	}
}
