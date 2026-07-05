//go:build integration

package applications

import (
	"context"
	"strings"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

// searchApp builds a baseline application for the search suite: a distinct uid
// from name (mirroring pgApp), plus an authority-scoped area id.
func searchApp(name, uid string, areaID int) PlanningApplication {
	a := pgApp(name, areaID)
	a.UID = uid
	return a
}

// TestPostgresStore_Search_ReferenceTierExactAndPrefix proves tier 1 matches on
// uid (NOT planit_name), case-insensitively, both exactly and by prefix.
func TestPostgresStore_Search_ReferenceTierExactAndPrefix(t *testing.T) {
	store := newAppPGStore(t)
	ctx := context.Background()

	target := searchApp("24/0099", "24/0099/FUL", 100)
	other := searchApp("24/0100", "24/0100/OUT", 100)
	for _, a := range []PlanningApplication{target, other} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	t.Run("exact match, case-insensitive", func(t *testing.T) {
		apps, refine, err := store.Search(ctx, "24/0099/ful", "", 20)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if refine {
			t.Error("refine: got true, want false")
		}
		if len(apps) != 1 || apps[0].Name != target.Name {
			t.Fatalf("got %+v, want exactly [%s]", apps, target.Name)
		}
	})

	t.Run("prefix match", func(t *testing.T) {
		apps, _, err := store.Search(ctx, "24/0099/F", "", 20)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(apps) != 1 || apps[0].Name != target.Name {
			t.Fatalf("got %+v, want exactly [%s]", apps, target.Name)
		}
	})

	t.Run("no match returns empty, not an error", func(t *testing.T) {
		apps, refine, err := store.Search(ctx, "no-such-reference-at-all", "", 20)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if refine {
			t.Error("refine: got true, want false")
		}
		if len(apps) != 0 {
			t.Fatalf("got %+v, want empty", apps)
		}
	})
}

// TestPostgresStore_Search_ReferenceTierCrossesAuthorities proves a bare
// reference legitimately returns rows from MULTIPLE authorities when no
// authority filter is given (application_uid is only unique within a council —
// tc-geq7h.3), and that the authority filter scopes it down to one.
func TestPostgresStore_Search_ReferenceTierCrossesAuthorities(t *testing.T) {
	store := newAppPGStore(t)
	ctx := context.Background()

	sharedUID := "shared-council-ref-24-0001"
	inA := searchApp("A1", sharedUID, 100)
	inB := searchApp("B1", sharedUID, 200)
	for _, a := range []PlanningApplication{inA, inB} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	unfiltered, _, err := store.Search(ctx, sharedUID, "", 20)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(unfiltered) != 2 {
		t.Fatalf("unfiltered: got %d rows, want 2 (one per authority)", len(unfiltered))
	}

	scoped, _, err := store.Search(ctx, sharedUID, "200", 20)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(scoped) != 1 || scoped[0].Name != inB.Name {
		t.Fatalf("scoped to authority 200: got %+v, want exactly [%s]", scoped, inB.Name)
	}
}

// TestPostgresStore_Search_AddressFuzzyMatch proves tier 2 finds a distinctive
// address fragment via pg_trgm word_similarity, case-insensitively, and does
// NOT match a clearly unrelated address.
func TestPostgresStore_Search_AddressFuzzyMatch(t *testing.T) {
	store := newAppPGStore(t)
	ctx := context.Background()

	match := searchApp("A1", "uid-a1", 100)
	match.Address = "42 Willowmere Gardens, Sometown"
	unrelated := searchApp("A2", "uid-a2", 100)
	unrelated.Address = "1 Zephyr Industrial Estate, Othertown"
	for _, a := range []PlanningApplication{match, unrelated} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	apps, _, err := store.Search(ctx, "willowmere", "", 20)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(apps) != 1 || apps[0].Name != match.Name {
		t.Fatalf("got %+v, want exactly [%s]", apps, match.Name)
	}
}

// TestPostgresStore_Search_DescriptionFullTextMatch proves tier 3 finds a
// description match via the english tsvector config, including basic English
// stemming (a query for "extension" matches a description containing
// "extensions").
func TestPostgresStore_Search_DescriptionFullTextMatch(t *testing.T) {
	store := newAppPGStore(t)
	ctx := context.Background()

	match := searchApp("D1", "uid-d1", 100)
	match.Description = "Two storey rear extensions and loft conversion"
	unrelated := searchApp("D2", "uid-d2", 100)
	unrelated.Description = "Felling of one protected oak tree"
	for _, a := range []PlanningApplication{match, unrelated} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	apps, _, err := store.Search(ctx, "extension", "", 20)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(apps) != 1 || apps[0].Name != match.Name {
		t.Fatalf("got %+v, want exactly [%s] (english stemming: extension -> extensions)", apps, match.Name)
	}
}

// TestPostgresStore_Search_RankingOrder proves the fixed tier order — reference
// > address > description — even though all three rows match the SAME query
// text, each via a different tier.
func TestPostgresStore_Search_RankingOrder(t *testing.T) {
	store := newAppPGStore(t)
	ctx := context.Background()

	const query = "hawthorn"

	byRef := searchApp("R1", query, 100) // tier 1: exact uid match
	byAddress := searchApp("R2", "uid-r2", 100)
	byAddress.Address = "9 Hawthorn Close, Sometown" // tier 2: address fuzzy match
	byDescription := searchApp("R3", "uid-r3", 100)
	byDescription.Description = "Removal of a hawthorn hedge" // tier 3: description match
	for _, a := range []PlanningApplication{byDescription, byAddress, byRef} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	apps, refine, err := store.Search(ctx, query, "", 20)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if refine {
		t.Error("refine: got true, want false")
	}
	if len(apps) != 3 {
		t.Fatalf("got %d rows, want 3; %+v", len(apps), apps)
	}
	gotOrder := []string{apps[0].Name, apps[1].Name, apps[2].Name}
	wantOrder := []string{byRef.Name, byAddress.Name, byDescription.Name}
	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Fatalf("rank order: got %v, want %v (reference > address > description)", gotOrder, wantOrder)
		}
	}
}

// TestPostgresStore_Search_LimitAndRefineFlag proves an over-limit match set is
// truncated to limit and reports refine=true (v1 has no cursor — tc-geq7h.3),
// while an exactly-at-limit set reports refine=false.
func TestPostgresStore_Search_LimitAndRefineFlag(t *testing.T) {
	store := newAppPGStore(t)
	ctx := context.Background()

	// 5 applications all sharing a distinctive description word.
	for i := range 5 {
		a := searchApp(nameForIndex(i), "uid-"+nameForIndex(i), 100)
		a.Description = "Demolition of existing outbuilding"
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	t.Run("limit below match count: truncated, refine true", func(t *testing.T) {
		apps, refine, err := store.Search(ctx, "demolition", "", 3)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(apps) != 3 {
			t.Fatalf("got %d rows, want 3 (truncated to limit)", len(apps))
		}
		if !refine {
			t.Error("refine: got false, want true (5 matches > limit 3)")
		}
	})

	t.Run("limit at or above match count: refine false", func(t *testing.T) {
		apps, refine, err := store.Search(ctx, "demolition", "", 5)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(apps) != 5 {
			t.Fatalf("got %d rows, want 5", len(apps))
		}
		if refine {
			t.Error("refine: got true, want false (5 matches == limit 5)")
		}
	})
}

func nameForIndex(i int) string {
	return "L" + string(rune('A'+i))
}

// TestPostgresStore_Search_BoundsEachTierWithLimitPushdown proves each of the
// three UNION ALL branches inside the matched CTE carries its own bounded
// ORDER BY ... LIMIT $3 (tc-z5i5j). Before this fix, only the outer SELECT had
// a LIMIT: because best's required DISTINCT ON ordering (area_id, planit_name,
// tier, score) differs from the final ORDER BY (tier, score, planit_name),
// Postgres could not push the final LIMIT down through DISTINCT ON, so any
// term the trigram/tsvector indexes consider common forced a full
// materialize-and-sort of the ENTIRE unbounded candidate set TWICE before the
// caller's limit was ever applied — the root cause of the prod incident
// (?q=extension: 15-53s). EXPLAIN (no ANALYZE — this needs no seeded rows and
// is cheap) must show a Limit node for each of the 4 LIMIT clauses in
// searchQuery (3 per-tier + 1 outer); fewer than 4 means a tier's bound was
// dropped and the slow, unbounded-materialization plan is back.
func TestPostgresStore_Search_BoundsEachTierWithLimitPushdown(t *testing.T) {
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "applications", "watch_zones")
	ctx := context.Background()

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire connection: %v", err)
	}
	defer conn.Release()

	var authorityArg any
	rows, err := conn.Query(ctx, "EXPLAIN (FORMAT TEXT) "+searchQuery, "extension", authorityArg, 21, "extension%")
	if err != nil {
		t.Fatalf("EXPLAIN: %v", err)
	}
	var plan strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			rows.Close()
			t.Fatalf("scan plan line: %v", err)
		}
		plan.WriteString(line)
		plan.WriteByte('\n')
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read plan: %v", err)
	}

	if got := strings.Count(plan.String(), "Limit"); got < 4 {
		t.Errorf("plan has %d Limit node(s), want >= 4 (3 per-tier + 1 outer):\n%s", got, plan.String())
	}
}

// TestPostgresStore_Search_ExplainUsesTrgmAndFTSIndexes proves the migration's
// GIN indexes (applications_address_trgm, applications_description_fts) serve
// the tier-2/tier-3 predicates — so the query never falls back to a sequential
// scan as the applications table grows. enable_seqscan is disabled for the
// EXPLAIN so the planner is forced to reveal whether each index is usable,
// mirroring TestPostgresStore_FindClustersInZone_ExplainUsesGiSTIndex.
func TestPostgresStore_Search_ExplainUsesTrgmAndFTSIndexes(t *testing.T) {
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "applications", "watch_zones")
	store := NewPostgresStore(pool)
	ctx := context.Background()
	for i := range 20 {
		a := searchApp(nameForIndex(i), "uid-"+nameForIndex(i), 100)
		a.Address = "Willowmere Gardens"
		a.Description = "a garden wall"
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire connection: %v", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SET enable_seqscan = off"); err != nil {
		t.Fatalf("disable seqscan: %v", err)
	}

	var authorityArg any
	rows, err := conn.Query(ctx, "EXPLAIN (FORMAT TEXT) "+searchQuery, "willowmere", authorityArg, 21, "willowmere%")
	if err != nil {
		t.Fatalf("EXPLAIN: %v", err)
	}
	var plan strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			rows.Close()
			t.Fatalf("scan plan line: %v", err)
		}
		plan.WriteString(line)
		plan.WriteByte('\n')
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read plan: %v", err)
	}

	planText := strings.ToLower(plan.String())
	if !strings.Contains(planText, "applications_address_trgm") {
		t.Errorf("EXPLAIN plan does not use applications_address_trgm:\n%s", plan.String())
	}
	if !strings.Contains(planText, "applications_description_fts") {
		t.Errorf("EXPLAIN plan does not use applications_description_fts:\n%s", plan.String())
	}
}
