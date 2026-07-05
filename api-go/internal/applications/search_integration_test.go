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

// TestPostgresStore_Search_TierCapPreservesRefineFlag proves the per-branch
// cap in searchQuery is genuinely limit+1, not limit (tc-z5i5j). It pins a
// property that is otherwise easy to get subtly wrong when hand-restructuring
// SQL: capping a branch at exactly `limit` rows can NEVER change which rows a
// caller actually sees (the returned page is always the top `limit` rows
// regardless, and a too-small cap only ever loses the row at rank limit+1 or
// later — which was always going to be truncated away) but it CAN silently
// flip RefineQuery from true to false, wrongly telling a client "no more
// results" when more genuinely exist. Two tier-1 matches (exact + prefix) for
// the same query, limit=1: the correct limit+1=2 cap keeps both, so Search
// sees 2 rows and correctly reports refine=true.
func TestPostgresStore_Search_TierCapPreservesRefineFlag(t *testing.T) {
	store := newAppPGStore(t)
	ctx := context.Background()

	exact := searchApp("T1", "onlyoneref", 100)
	prefix := searchApp("T2", "onlyonerefplus", 100)
	for _, a := range []PlanningApplication{exact, prefix} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	apps, refine, err := store.Search(ctx, "onlyoneref", "", 1)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(apps) != 1 || apps[0].Name != exact.Name {
		t.Fatalf("got %+v, want exactly [%s] (exact match ranks first)", apps, exact.Name)
	}
	if !refine {
		t.Error("refine: got false, want true — 2 tier-1 matches exist beyond limit 1; " +
			"a branch cap of `limit` instead of `limit+1` would silently drop the prefix " +
			"match and wrongly report refine=false")
	}
}

// TestPostgresStore_Search_CrossTierCrowdingPreservesGenuineWinners proves the
// per-branch cap is correct even when a higher-tier duplicate occupies a slot
// in a lower tier's own ranking (tc-z5i5j) — the trickiest part of the
// restructuring's correctness argument (see the doc comment on searchQuery).
//
// TA matches BOTH tier 1 (exact reference) and tier 2 (its own address is the
// single best address-fuzzy match for the query, word_similarity 1.0) — so
// within the raw tier-2 branch, before any deduplication, TA occupies the
// #1-by-score slot. DISTINCT ON then keeps TA under tier 1 (its
// higher-priority match), discarding TA's tier-2 duplicate. TC and TD are
// address-only matches (word_similarity 0.8 and 0.7) that never match tier 1
// at all — the genuine tier-2 winners. TF is a fourth, weaker address match
// (word_similarity 0.6) that must legitimately be excluded from the top
// limit+1=3 results.
//
// With limit=2 (limit+1=3), the tier-2 branch's raw top-3 by score is exactly
// {TA, TC, TD} — TA's crowded #1 slot still leaves room for both genuine
// winners TC and TD, and TF is correctly excluded. If the branch were instead
// capped at `limit` (2, one too few), TA's crowding would push TD out of the
// branch entirely: the DISTINCT ON row count would drop from 3 to 2, and
// RefineQuery would wrongly flip to false even though a third match (TD)
// genuinely exists.
func TestPostgresStore_Search_CrossTierCrowdingPreservesGenuineWinners(t *testing.T) {
	store := newAppPGStore(t)
	ctx := context.Background()

	const query = "crowdterm"

	ta := searchApp("CA", query, 100) // tier 1: exact uid match
	ta.Address = "42 Crowdterm Way, Sometown"
	tc := searchApp("CC", "uid-cc", 100)
	tc.Address = "1 Acrowdterm Gardens, Sometown"
	td := searchApp("CD", "uid-cd", 100)
	td.Address = "99 Crowdte Close, Sometown"
	tf := searchApp("CF", "uid-cf", 100)
	tf.Address = "7 Crowdt Row, Sometown"
	for _, a := range []PlanningApplication{ta, tc, td, tf} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	apps, refine, err := store.Search(ctx, query, "", 2)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("got %d rows, want 2 (truncated to limit); %+v", len(apps), apps)
	}
	gotOrder := []string{apps[0].Name, apps[1].Name}
	wantOrder := []string{ta.Name, tc.Name}
	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Fatalf("rank order: got %v, want %v (tier1 TA, then genuine tier2 winner TC — "+
				"not crowded out, and not TF)", gotOrder, wantOrder)
		}
	}
	if !refine {
		t.Error("refine: got false, want true — TD is a genuine third match beyond limit 2; " +
			"an undersized tier-2 cap would let TA's crowding push TD out entirely")
	}
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

// TestPostgresStore_Search_TiedScoresStayDeterministicUnderCap proves the
// per-branch cap added for tc-z5i5j does not regress determinism when many
// rows in one tier tie on score. Each branch's own "ORDER BY score DESC
// LIMIT $3" needs a secondary "planit_name ASC" key matching the final
// query's tie-break — without it, Postgres is free to keep an ARBITRARY
// limit+1-sized subset of the tied rows (whichever the scan happens to visit
// first), and the final ORDER BY's planit_name tie-break can only resolve
// ties among whatever subset survived the branch cap, not the true full set.
// 50 applications share one address (identical word_similarity, so all tied
// at tier 2), seeded in reverse-name order to catch any reliance on
// insertion/scan order: the correct, deterministic answer is always the
// alphabetically-first 10 names, byte-for-byte what the old unbounded query
// would have returned.
func TestPostgresStore_Search_TiedScoresStayDeterministicUnderCap(t *testing.T) {
	store := newAppPGStore(t)
	ctx := context.Background()

	const n = 50
	for i := n - 1; i >= 0; i-- {
		name := tieName(i)
		a := searchApp(name, "uid-"+name, 100)
		a.Address = "1 Tieword Lane, Sometown"
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	apps, refine, err := store.Search(ctx, "tieword", "", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(apps) != 10 {
		t.Fatalf("got %d rows, want 10 (truncated to limit)", len(apps))
	}
	if !refine {
		t.Error("refine: got false, want true (50 tied matches > limit 10)")
	}
	for i, a := range apps {
		if want := tieName(i); a.Name != want {
			t.Errorf("rank %d: got %q, want %q (alphabetically-first tied name)", i, a.Name, want)
		}
	}
}

// tieName generates deterministic, lexicographically-ordered names ("TIE-AA",
// "TIE-AB", ...) for the tied-score determinism test above.
func tieName(i int) string {
	return "TIE-" + string(rune('A'+i/26)) + string(rune('A'+i%26))
}

// TestPostgresStore_Search_LargeScaleCrossTierCandidateSet reproduces the prod
// incident directly (tc-z5i5j: ?q=extension took 15-53s, sometimes timing out
// outright) with a seeded dataset large enough that "extension" genuinely
// exercises a large candidate set across all three tiers, not just a
// synthetic few-row fixture. It asserts exact top-K correctness — the same
// result the old, unbounded, correct-but-slow query would have produced: tier
// 1 first (exact, then the two tied prefix matches broken by planit_name),
// then the alphabetically-first tier-2 winners, tier 3 never reached because
// 300 tier-2 matches alone are far more than enough to fill the remaining
// slots. The elapsed-time assertion is a generous, CI-tolerant smoke
// trip-wire only; the primary proof that the query no longer needs a full
// sort of the unbounded candidate set is the structural, non-flaky EXPLAIN
// check in TestPostgresStore_Search_BoundsEachTierWithLimitPushdown.
func TestPostgresStore_Search_LargeScaleCrossTierCandidateSet(t *testing.T) {
	store := newAppPGStore(t)
	ctx := context.Background()

	const perTier = 300

	// Tier 1: one exact match plus two prefix matches, tied at score 1.0 and
	// broken by planit_name (P-AA before P-AB).
	exact := searchApp("E-EXACT", "extension", 100)
	prefixA := searchApp("P-AA", "extension-annex", 100)
	prefixB := searchApp("P-AB", "extension-loft", 100)
	for _, a := range []PlanningApplication{exact, prefixA, prefixB} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	// Tier 2: perTier applications whose address all fuzzy-match "extension"
	// with an IDENTICAL word_similarity score (tied) — none also match tier 1
	// (uid is unrelated) or tier 3 (description is the pgApp default).
	for i := range perTier {
		name := "T2-" + fmt.Sprintf("%03d", i)
		a := searchApp(name, "addr-uid-"+name, 100)
		a.Address = "10 Extension Close, Sometown"
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	// Tier 3: perTier applications whose description all full-text-match
	// "extension" with an IDENTICAL ts_rank (tied) — none also match tier 1
	// (uid is unrelated) or tier 2 (address is the pgApp default).
	for i := range perTier {
		name := "T3-" + fmt.Sprintf("%03d", i)
		a := searchApp(name, "desc-uid-"+name, 100)
		a.Description = "Two storey rear extension"
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	start := time.Now()
	apps, refine, err := store.Search(ctx, "extension", "", 20)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if elapsed > 3*time.Second {
		t.Errorf("Search took %v against %d rows spanning all 3 tiers — want comfortably sub-second; "+
			"this is a smoke trip-wire only, see TestPostgresStore_Search_BoundsEachTierWithLimitPushdown "+
			"for the primary structural proof", elapsed, 3+2*perTier)
	}

	if len(apps) != 20 {
		t.Fatalf("got %d rows, want 20 (truncated to limit); names=%v", len(apps), namesOf(apps))
	}
	if !refine {
		t.Error("refine: got false, want true (far more than 20 matches exist across all 3 tiers)")
	}

	wantOrder := []string{"E-EXACT", "P-AA", "P-AB"}
	for i := 0; i < 17; i++ {
		wantOrder = append(wantOrder, fmt.Sprintf("T2-%03d", i))
	}
	if got := namesOf(apps); !equalNames(got, wantOrder) {
		t.Fatalf("rank order:\n got  %v\n want %v (tier 1 first, then the 17 alphabetically-first "+
			"tier-2 winners needed to fill limit+1=21 before truncation; tier 3 never reached)", got, wantOrder)
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
