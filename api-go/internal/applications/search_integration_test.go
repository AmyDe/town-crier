//go:build integration

package applications

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

// searchApp builds a baseline application for the search suite: a distinct uid
// from name (mirroring pgApp), plus an authority-scoped area id. It has no
// coordinates — callers place it with at() (store_postgres_test.go), since
// GH#863 made location mandatory for every match tier (a nearest-first search
// has no meaning for an application PlanIt never geocoded).
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

	target := at(searchApp("24/0099", "24/0099/FUL", 100), 100)
	other := at(searchApp("24/0100", "24/0100/OUT", 100), 200)
	for _, a := range []PlanningApplication{target, other} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	t.Run("exact match, case-insensitive", func(t *testing.T) {
		apps, refine, err := store.Search(ctx, "24/0099/ful", "", pgCentreLat, pgCentreLon, 20)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if refine {
			t.Error("refine: got true, want false")
		}
		// Reference must stay planit_name (Name), never uid — a real regression
		// once (tc-geq7h.3); the fixture's uid ("24/0099/FUL") differs from its
		// name ("24/0099") specifically to catch it.
		if len(apps) != 1 || apps[0].Name != target.Name {
			t.Fatalf("got %+v, want exactly [%s]", apps, target.Name)
		}
	})

	t.Run("prefix match", func(t *testing.T) {
		apps, _, err := store.Search(ctx, "24/0099/F", "", pgCentreLat, pgCentreLon, 20)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(apps) != 1 || apps[0].Name != target.Name {
			t.Fatalf("got %+v, want exactly [%s]", apps, target.Name)
		}
	})

	t.Run("no match returns empty, not an error", func(t *testing.T) {
		apps, refine, err := store.Search(ctx, "no-such-reference-at-all", "", pgCentreLat, pgCentreLon, 20)
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
	inA := at(searchApp("A1", sharedUID, 100), 100)
	inB := at(searchApp("B1", sharedUID, 200), 200)
	for _, a := range []PlanningApplication{inA, inB} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	unfiltered, _, err := store.Search(ctx, sharedUID, "", pgCentreLat, pgCentreLon, 20)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(unfiltered) != 2 {
		t.Fatalf("unfiltered: got %d rows, want 2 (one per authority)", len(unfiltered))
	}

	scoped, _, err := store.Search(ctx, sharedUID, "200", pgCentreLat, pgCentreLon, 20)
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

	match := at(searchApp("A1", "uid-a1", 100), 100)
	match.Address = "42 Willowmere Gardens, Sometown"
	unrelated := at(searchApp("A2", "uid-a2", 100), 200)
	unrelated.Address = "1 Zephyr Industrial Estate, Othertown"
	for _, a := range []PlanningApplication{match, unrelated} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	apps, _, err := store.Search(ctx, "willowmere", "", pgCentreLat, pgCentreLon, 20)
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

	match := at(searchApp("D1", "uid-d1", 100), 100)
	match.Description = "Two storey rear extensions and loft conversion"
	unrelated := at(searchApp("D2", "uid-d2", 100), 200)
	unrelated.Description = "Felling of one protected oak tree"
	for _, a := range []PlanningApplication{match, unrelated} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	apps, _, err := store.Search(ctx, "extension", "", pgCentreLat, pgCentreLon, 20)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(apps) != 1 || apps[0].Name != match.Name {
		t.Fatalf("got %+v, want exactly [%s] (english stemming: extension -> extensions)", apps, match.Name)
	}
}

// TestPostgresStore_Search_RankingOrder proves the GH#863 redesign's core
// behaviour change: ranking is distance-first, NOT tier-priority. Before
// GH#863 this test asserted a fixed reference > address > description order
// regardless of distance — that assertion is now WRONG BY DESIGN. A NEARBY
// tier-2 match must outrank a FARTHER tier-1 match: byRef (tier 1, exact uid
// match) sits 1000m away, byAddress (tier 2, address fuzzy match) sits only
// 50m away — closer wins, even though it matched a lower-priority tier under
// the old scheme.
func TestPostgresStore_Search_RankingOrder(t *testing.T) {
	store := newAppPGStore(t)
	ctx := context.Background()

	const query = "hawthorn"

	byRef := at(searchApp("R1", query, 100), 1000)      // tier 1, far
	byAddress := at(searchApp("R2", "uid-r2", 100), 50) // tier 2, near
	byAddress.Address = "9 Hawthorn Close, Sometown"
	for _, a := range []PlanningApplication{byRef, byAddress} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	apps, refine, err := store.Search(ctx, query, "", pgCentreLat, pgCentreLon, 20)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if refine {
		t.Error("refine: got true, want false")
	}
	if len(apps) != 2 {
		t.Fatalf("got %d rows, want 2; %+v", len(apps), apps)
	}
	wantOrder := []string{byAddress.Name, byRef.Name}
	if got := namesOf(apps); !equalNames(got, wantOrder) {
		t.Fatalf("rank order: got %v, want %v (nearer tier-2 match beats farther tier-1 match — "+
			"distance-first, not tier-priority)", got, wantOrder)
	}
}

// TestPostgresStore_Search_NearestFirstAcrossAllThreeTiers extends the
// distance-first proof to all three tiers at once: three applications, each
// matching a DIFFERENT tier for the same query text, placed at distances that
// invert the old tier-priority order entirely (tier 3 nearest, tier 1
// farthest) — the returned order must be exactly nearest-to-farthest.
func TestPostgresStore_Search_NearestFirstAcrossAllThreeTiers(t *testing.T) {
	store := newAppPGStore(t)
	ctx := context.Background()

	const query = "crocus"

	byDescription := at(searchApp("N1", "uid-n1", 100), 100) // tier 3, nearest
	byDescription.Description = "Removal of a crocus border"
	byAddress := at(searchApp("N2", "uid-n2", 100), 500) // tier 2, middle
	byAddress.Address = "3 Crocus Way, Sometown"
	byRef := at(searchApp("N3", query, 100), 900) // tier 1, farthest
	for _, a := range []PlanningApplication{byRef, byAddress, byDescription} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	apps, refine, err := store.Search(ctx, query, "", pgCentreLat, pgCentreLon, 20)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if refine {
		t.Error("refine: got true, want false")
	}
	wantOrder := []string{byDescription.Name, byAddress.Name, byRef.Name}
	if got := namesOf(apps); !equalNames(got, wantOrder) {
		t.Fatalf("rank order: got %v, want %v (nearest-first across all 3 tiers, "+
			"regardless of which tier matched)", got, wantOrder)
	}
}

// TestPostgresStore_Search_TieBreakAtEqualDistance proves the planit_name
// secondary sort key: many applications tied at the exact SAME distance
// (identical coordinates) must come back in deterministic, alphabetically
// ascending planit_name order — not whatever arbitrary order the scan or the
// Go-side merge/dedup happens to visit them in. 50 applications share one
// address (so all tie in tier 2) AND one point (so all tie at distance 0),
// seeded in reverse-name order to catch any reliance on insertion or scan
// order.
func TestPostgresStore_Search_TieBreakAtEqualDistance(t *testing.T) {
	store := newAppPGStore(t)
	ctx := context.Background()

	const n = 50
	for i := n - 1; i >= 0; i-- {
		name := tieName(i)
		a := at(searchApp(name, "uid-"+name, 100), 100)
		a.Address = "1 Tieword Lane, Sometown"
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	apps, refine, err := store.Search(ctx, "tieword", "", pgCentreLat, pgCentreLon, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(apps) != 10 {
		t.Fatalf("got %d rows, want 10 (truncated to limit)", len(apps))
	}
	if !refine {
		t.Error("refine: got false, want true (50 tied-distance matches > limit 10)")
	}
	for i, a := range apps {
		if want := tieName(i); a.Name != want {
			t.Errorf("rank %d: got %q, want %q (alphabetically-first tied name)", i, a.Name, want)
		}
	}
}

func tieName(i int) string {
	return "TIE-" + string(rune('A'+i/26)) + string(rune('A'+i%26))
}

// TestPostgresStore_Search_LimitAndRefineFlag proves an over-limit match set is
// truncated to limit and reports refine=true (v1 has no cursor — tc-geq7h.3),
// while an exactly-at-limit set reports refine=false.
func TestPostgresStore_Search_LimitAndRefineFlag(t *testing.T) {
	store := newAppPGStore(t)
	ctx := context.Background()

	// 5 applications, all matching on description, spread at distinct
	// distances so nearest-first order is unambiguous.
	for i := range 5 {
		a := at(searchApp(nameForIndex(i), "uid-"+nameForIndex(i), 100), float64(100*(i+1)))
		a.Description = "Demolition of existing outbuilding"
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	t.Run("limit below match count: truncated, refine true", func(t *testing.T) {
		apps, refine, err := store.Search(ctx, "demolition", "", pgCentreLat, pgCentreLon, 3)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(apps) != 3 {
			t.Fatalf("got %d rows, want 3 (truncated to limit)", len(apps))
		}
		if !refine {
			t.Error("refine: got false, want true (5 matches > limit 3)")
		}
		wantOrder := []string{nameForIndex(0), nameForIndex(1), nameForIndex(2)}
		if got := namesOf(apps); !equalNames(got, wantOrder) {
			t.Fatalf("rank order: got %v, want %v (nearest 3)", got, wantOrder)
		}
	})

	t.Run("limit at or above match count: refine false", func(t *testing.T) {
		apps, refine, err := store.Search(ctx, "demolition", "", pgCentreLat, pgCentreLon, 5)
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

// TestPostgresStore_Search_PerTierCapPreservesRefineFlag proves the per-tier
// LIMIT (limit+1, not limit) is genuinely load-bearing for RefineQuery under
// the new distance-first merge (tc-z5i5j's original concern, re-derived for
// distance instead of tier/score — see the doc comment on searchTier1Query).
// Two tier-1 matches (exact + prefix) tied at the SAME distance, limit=1: the
// correct limit+1=2 per-tier cap keeps both, so Search sees 2 distinct rows
// and correctly reports refine=true.
func TestPostgresStore_Search_PerTierCapPreservesRefineFlag(t *testing.T) {
	store := newAppPGStore(t)
	ctx := context.Background()

	exact := at(searchApp("T1", "onlyoneref", 100), 100)
	prefix := at(searchApp("T2", "onlyonerefplus", 100), 100)
	for _, a := range []PlanningApplication{exact, prefix} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	apps, refine, err := store.Search(ctx, "onlyoneref", "", pgCentreLat, pgCentreLon, 1)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(apps) != 1 || apps[0].Name != exact.Name {
		t.Fatalf("got %+v, want exactly [%s] (tied distance -> planit_name tie-break: T1 < T2)", apps, exact.Name)
	}
	if !refine {
		t.Error("refine: got false, want true — 2 tier-1 matches exist beyond limit 1; " +
			"a per-tier cap of `limit` instead of `limit+1` would silently drop the prefix " +
			"match and wrongly report refine=false")
	}
}

// TestPostgresStore_Search_CrossTierDuplicateCountsOnce proves an application
// matching MORE THAN ONE tier for the same query is deduplicated to a single
// row in the merged result, never returned twice and never double-counted
// toward RefineQuery. crossMatch matches both tier 1 (exact uid) and tier 2
// (its own address); alone with the query's limit set to exactly the number
// of GENUINE distinct matches, RefineQuery must be false — if the dedup were
// missing or broken, the duplicate would inflate the merged set past the
// limit and wrongly flip RefineQuery to true.
func TestPostgresStore_Search_CrossTierDuplicateCountsOnce(t *testing.T) {
	store := newAppPGStore(t)
	ctx := context.Background()

	const query = "duplimatch"

	crossMatch := at(searchApp("X1", query, 100), 100) // tier 1 AND tier 2
	crossMatch.Address = "1 Duplimatch Road, Sometown"
	other := at(searchApp("X2", "uid-x2", 100), 200) // tier 2 only
	other.Address = "2 Duplimatch Road, Sometown"
	for _, a := range []PlanningApplication{crossMatch, other} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	apps, refine, err := store.Search(ctx, query, "", pgCentreLat, pgCentreLon, 2)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("got %d rows, want 2 distinct applications (crossMatch deduplicated to one row); %+v",
			len(apps), namesOf(apps))
	}
	wantOrder := []string{crossMatch.Name, other.Name}
	if got := namesOf(apps); !equalNames(got, wantOrder) {
		t.Fatalf("rank order: got %v, want %v", got, wantOrder)
	}
	if refine {
		t.Error("refine: got true, want false — exactly 2 genuine distinct matches exist, " +
			"a broken dedup counting crossMatch twice would wrongly report more")
	}
}

// TestPostgresStore_Search_ExplainUsesExpectedIndexes proves each of the three
// tier queries is served by its expected index rather than a sequential scan
// as the applications table grows: the KNN GiST index (applications_
// location_gist) orders every tier by distance, and each tier's own
// text-match predicate is served by its dedicated index (applications_uid_
// lower_pattern for tier 1, applications_address_trgm for tier 2,
// applications_description_fts for tier 3). enable_seqscan is disabled for the
// EXPLAIN so the planner is forced to reveal whether each index is usable,
// mirroring TestPostgresStore_FindClustersInZone_ExplainUsesGiSTIndex.
func TestPostgresStore_Search_ExplainUsesExpectedIndexes(t *testing.T) {
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "applications", "watch_zones")
	store := NewPostgresStore(pool)
	ctx := context.Background()
	for i := range 20 {
		a := at(searchApp(nameForIndex(i), "uid-"+nameForIndex(i), 100), float64(100*(i+1)))
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

	explainPlan := func(t *testing.T, query string, args ...any) string {
		t.Helper()
		rows, err := conn.Query(ctx, "EXPLAIN (FORMAT TEXT) "+query, args...)
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
		return plan.String()
	}

	var authorityArg any

	t.Run("tier 1 uses uid pattern index and shows a Limit node", func(t *testing.T) {
		plan := strings.ToLower(explainPlan(t, searchTier1Query,
			pgCentreLon, pgCentreLat, "willowmere", "willowmere%", authorityArg, 21))
		if !strings.Contains(plan, "applications_uid_lower_pattern") {
			t.Errorf("EXPLAIN plan does not use applications_uid_lower_pattern:\n%s", plan)
		}
		if !strings.Contains(plan, "limit") {
			t.Errorf("EXPLAIN plan has no Limit node:\n%s", plan)
		}
	})

	t.Run("tier 2 uses address trgm index and shows a Limit node", func(t *testing.T) {
		plan := strings.ToLower(explainPlan(t, searchTier2Query,
			pgCentreLon, pgCentreLat, "willowmere", authorityArg, 21))
		if !strings.Contains(plan, "applications_address_trgm") {
			t.Errorf("EXPLAIN plan does not use applications_address_trgm:\n%s", plan)
		}
		if !strings.Contains(plan, "limit") {
			t.Errorf("EXPLAIN plan has no Limit node:\n%s", plan)
		}
	})

	t.Run("tier 3 uses description fts index and shows a Limit node", func(t *testing.T) {
		plan := strings.ToLower(explainPlan(t, searchTier3Query,
			pgCentreLon, pgCentreLat, "wall", authorityArg, 21))
		if !strings.Contains(plan, "applications_description_fts") {
			t.Errorf("EXPLAIN plan does not use applications_description_fts:\n%s", plan)
		}
		if !strings.Contains(plan, "limit") {
			t.Errorf("EXPLAIN plan has no Limit node:\n%s", plan)
		}
	})
}

// TestPostgresStore_Search_LargeScaleCrossTierCandidateSet reproduces the
// prod incident's shape (tc-z5i5j: ?q=extension took 15-53s against the old
// tier-priority CTE/UNION query) at a smaller but still multi-tier,
// multi-hundred-row scale, proving the KNN per-tier-query rewrite stays fast
// and exactly correct: nearest-first order must hold across every tier at
// once, not just within one.
func TestPostgresStore_Search_LargeScaleCrossTierCandidateSet(t *testing.T) {
	store := newAppPGStore(t)
	ctx := context.Background()

	const perTier = 60

	// Tier 1: one exact match plus two prefix matches, each placed at a
	// distinct, deliberately FAR distance so a correct implementation must
	// rank them behind every one of the (much nearer) tier-2/tier-3 matches
	// below — proving distance, not tier, drives the order.
	exact := at(searchApp("E-EXACT", "extension", 100), 100_000)
	prefixA := at(searchApp("P-AA", "extension-annex", 100), 100_100)
	prefixB := at(searchApp("P-AB", "extension-loft", 100), 100_200)
	for _, a := range []PlanningApplication{exact, prefixA, prefixB} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	// Tier 2: perTier applications, all matching "extension" via address
	// fuzzy match, placed at ascending distances 100m, 200m, ... — the
	// nearest perTier matches overall.
	for i := range perTier {
		name := "T2-" + fmt.Sprintf("%03d", i)
		a := at(searchApp(name, "addr-uid-"+name, 100), float64(100*(i+1)))
		a.Address = "10 Extension Close, Sometown"
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	// Tier 3: perTier applications matching "extension" via description,
	// placed FARTHER than every tier-2 match but nearer than tier 1.
	for i := range perTier {
		name := "T3-" + fmt.Sprintf("%03d", i)
		a := at(searchApp(name, "desc-uid-"+name, 100), float64(50_000+100*(i+1)))
		a.Description = "Two storey rear extension"
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	start := time.Now()
	apps, refine, err := store.Search(ctx, "extension", "", pgCentreLat, pgCentreLon, 20)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if elapsed > 3*time.Second {
		t.Errorf("Search took %v against %d rows spanning all 3 tiers — want comfortably sub-second",
			elapsed, 3+2*perTier)
	}

	if len(apps) != 20 {
		t.Fatalf("got %d rows, want 20 (truncated to limit); names=%v", len(apps), namesOf(apps))
	}
	if !refine {
		t.Error("refine: got false, want true (far more than 20 matches exist across all 3 tiers)")
	}

	// The 20 nearest overall are exactly the 20 nearest tier-2 matches
	// (T2-000..T2-019): every tier-2 match is nearer than every tier-3 match,
	// which is in turn nearer than every tier-1 match.
	wantOrder := make([]string, 20)
	for i := range wantOrder {
		wantOrder[i] = fmt.Sprintf("T2-%03d", i)
	}
	if got := namesOf(apps); !equalNames(got, wantOrder) {
		t.Fatalf("rank order:\n got  %v\n want %v (nearest-first: all 20 nearest tier-2 matches, "+
			"tier 1/3 never reached despite being valid matches)", got, wantOrder)
	}
}

func namesOf(apps []PlanningApplication) []string {
	names := make([]string, len(apps))
	for i, a := range apps {
		names[i] = a.Name
	}
	return names
}

func equalNames(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
