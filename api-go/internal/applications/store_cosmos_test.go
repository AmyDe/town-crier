package applications

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

type fakeItems struct {
	stored          map[string][]byte // id -> bytes
	readErr         error
	upsertErr       error
	queryErr        error
	queryResult     [][]byte
	lastUpsertPK    string
	lastUpsert      []byte
	lastReadPK      string
	lastReadID      string
	lastQueryPK     string
	lastQuery       string
	lastQueryParams map[string]any
	// lastQueryViaLongRead records whether the most recent query went through the
	// longer-budget build-read accessor (QueryItemsLongRead) rather than the tight
	// OLTP QueryItems. The build-time SEO reads must use the long-read path; the
	// user-facing reads must not (tc-9tov).
	lastQueryViaLongRead bool
}

func newFakeItems() *fakeItems { return &fakeItems{stored: map[string][]byte{}} }

func (f *fakeItems) ReadItem(_ context.Context, partitionKey, id string) ([]byte, error) {
	f.lastReadPK = partitionKey
	f.lastReadID = id
	if f.readErr != nil {
		return nil, f.readErr
	}
	b, ok := f.stored[id]
	if !ok {
		return nil, &azcore.ResponseError{StatusCode: http.StatusNotFound}
	}
	return b, nil
}

func (f *fakeItems) UpsertItem(_ context.Context, partitionKey string, item []byte) error {
	f.lastUpsertPK = partitionKey
	f.lastUpsert = item
	return f.upsertErr
}

func (f *fakeItems) QueryItems(_ context.Context, partitionKey, query string, params map[string]any) ([][]byte, error) {
	f.lastQueryViaLongRead = false
	return f.recordQuery(partitionKey, query, params)
}

func (f *fakeItems) QueryItemsLongRead(_ context.Context, partitionKey, query string, params map[string]any) ([][]byte, error) {
	f.lastQueryViaLongRead = true
	return f.recordQuery(partitionKey, query, params)
}

func (f *fakeItems) recordQuery(partitionKey, query string, params map[string]any) ([][]byte, error) {
	f.lastQueryPK = partitionKey
	f.lastQuery = query
	f.lastQueryParams = params
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	// Simulate Cosmos honouring SELECT TOP @cap: a query that binds @cap returns
	// at most that many rows, so a busy-authority partition is truncated server-
	// side rather than in the store.
	if capRaw, ok := params["@cap"]; ok {
		if capN, ok := capRaw.(int); ok && capN >= 0 && capN < len(f.queryResult) {
			return f.queryResult[:capN], nil
		}
	}
	return f.queryResult, nil
}

func TestCosmosStore_Upsert_TargetsAuthorityPartition(t *testing.T) {
	t.Parallel()
	a := testApplication(t)
	items := newFakeItems()
	store := NewCosmosStore(items)

	if err := store.Upsert(context.Background(), a); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if items.lastUpsertPK != strconv.Itoa(a.AreaID) {
		t.Errorf("upsert partition key: got %q, want %q", items.lastUpsertPK, strconv.Itoa(a.AreaID))
	}
	var doc applicationDocument
	if err := json.Unmarshal(items.lastUpsert, &doc); err != nil {
		t.Fatalf("decode upserted doc: %v", err)
	}
	if doc.ID != a.Name || doc.AuthorityCode != strconv.Itoa(a.AreaID) {
		t.Errorf("upserted doc identity: got id=%q authorityCode=%q", doc.ID, doc.AuthorityCode)
	}
}

func TestCosmosStore_GetByAuthorityAndName_PointRead(t *testing.T) {
	t.Parallel()
	a := testApplication(t)
	items := newFakeItems()
	body, _ := json.Marshal(newApplicationDocument(a))
	items.stored[a.Name] = body
	store := NewCosmosStore(items)

	got, found, err := store.GetByAuthorityAndName(context.Background(), strconv.Itoa(a.AreaID), a.Name)
	if err != nil {
		t.Fatalf("GetByAuthorityAndName: %v", err)
	}
	if !found {
		t.Fatal("expected found")
	}
	if got.Name != a.Name || got.UID != a.UID {
		t.Errorf("got %+v", got)
	}
	if items.lastReadPK != strconv.Itoa(a.AreaID) || items.lastReadID != a.Name {
		t.Errorf("point read keys: pk=%q id=%q", items.lastReadPK, items.lastReadID)
	}
}

func TestCosmosStore_GetByAuthorityAndName_NotFound(t *testing.T) {
	t.Parallel()
	store := NewCosmosStore(newFakeItems())
	_, found, err := store.GetByAuthorityAndName(context.Background(), "471", "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected not found")
	}
}

func TestCosmosStore_GetByUID_PartitionScopedQuery(t *testing.T) {
	t.Parallel()
	a := testApplication(t)
	body, _ := json.Marshal(newApplicationDocument(a))
	items := newFakeItems()
	items.queryResult = [][]byte{body}
	store := NewCosmosStore(items)

	got, found, err := store.GetByUID(context.Background(), a.UID, strconv.Itoa(a.AreaID))
	if err != nil {
		t.Fatalf("GetByUID: %v", err)
	}
	if !found {
		t.Fatal("expected found")
	}
	if got.UID != a.UID || got.Name != a.Name {
		t.Errorf("got %+v", got)
	}
	if items.lastQueryPK != strconv.Itoa(a.AreaID) {
		t.Errorf("query partition key: got %q, want %q", items.lastQueryPK, strconv.Itoa(a.AreaID))
	}
	want := "SELECT * FROM c WHERE c.uid = @uid"
	if items.lastQuery != want {
		t.Errorf("uid query:\n got %q\nwant %q", items.lastQuery, want)
	}
}

func TestCosmosStore_GetByUID_NotFound(t *testing.T) {
	t.Parallel()
	store := NewCosmosStore(newFakeItems())

	_, found, err := store.GetByUID(context.Background(), "missing-uid", "471")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected not found")
	}
}

func TestCosmosStore_FindNearby_ScopesToAuthorityPartitionWithSpatialQuery(t *testing.T) {
	t.Parallel()
	a := testApplication(t)
	body, _ := json.Marshal(newApplicationDocument(a))
	items := newFakeItems()
	items.queryResult = [][]byte{body}
	store := NewCosmosStore(items)

	got, err := store.FindNearby(context.Background(), "441", 51.4975, -0.1357, 2000)
	if err != nil {
		t.Fatalf("FindNearby: %v", err)
	}
	if len(got) != 1 || got[0].Name != a.Name {
		t.Fatalf("FindNearby results: got %+v", got)
	}
	if items.lastQueryPK != "441" {
		t.Errorf("query partition key: got %q, want \"441\"", items.lastQueryPK)
	}
	// The GeoJSON point carries [longitude, latitude] (GeoJSON order) and the
	// radius is a named parameter, mirroring findZonesContainingQuery style.
	want := "SELECT * FROM c WHERE ST_DISTANCE(c.location, " +
		`{"type": "Point", "coordinates": [@longitude, @latitude]}) <= @radiusMetres`
	if items.lastQuery != want {
		t.Errorf("spatial query:\n got %q\nwant %q", items.lastQuery, want)
	}
}

func TestFindNearby_UsesParameterizedQuery(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	store := NewCosmosStore(items)

	_, _ = store.FindNearby(context.Background(), "441", 51.4975, -0.1357, 2000)

	// The query text must not contain any float literal — values must be bound
	// as named parameters, not string-concatenated.
	if items.lastQuery == "" {
		t.Fatal("no query was issued")
	}
	for _, literal := range []string{"51.4975", "-0.1357", "2000"} {
		if strings.Contains(items.lastQuery, literal) {
			t.Errorf("query contains float literal %q — should be a named parameter; query: %s", literal, items.lastQuery)
		}
	}
	// Verify the three expected params are bound with correct values.
	wantParams := map[string]any{
		"@latitude":     51.4975,
		"@longitude":    -0.1357,
		"@radiusMetres": 2000.0,
	}
	for k, wantVal := range wantParams {
		gotVal, ok := items.lastQueryParams[k]
		if !ok {
			t.Errorf("param %q not found in query params %v", k, items.lastQueryParams)
			continue
		}
		if gotVal != wantVal {
			t.Errorf("param %q: got %v, want %v", k, gotVal, wantVal)
		}
	}
}

func TestCosmosStore_FindNearby_EmptyResultIsEmptySlice(t *testing.T) {
	t.Parallel()
	store := NewCosmosStore(newFakeItems())

	got, err := store.FindNearby(context.Background(), "441", 51.4975, -0.1357, 2000)
	if err != nil {
		t.Fatalf("FindNearby: %v", err)
	}
	if got == nil {
		t.Fatal("FindNearby: got nil slice, want empty non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("FindNearby: got %d results, want 0", len(got))
	}
}

// recentDocs marshals n application documents whose Name (and so id) is unique,
// for the busy-authority cap test.
func recentDocs(t *testing.T, n int) [][]byte {
	t.Helper()
	a := testApplication(t)
	docs := make([][]byte, 0, n)
	for i := range n {
		a.Name = "24/" + strconv.Itoa(i) + "/FUL"
		body, err := json.Marshal(newApplicationDocument(a))
		if err != nil {
			t.Fatalf("marshal doc %d: %v", i, err)
		}
		docs = append(docs, body)
	}
	return docs
}

func TestCosmosStore_RecentByAuthority_TopNOrderedScopedToPartition(t *testing.T) {
	t.Parallel()
	a := testApplication(t)
	body, _ := json.Marshal(newApplicationDocument(a))
	items := newFakeItems()
	items.queryResult = [][]byte{body}
	store := NewCosmosStore(items)

	got, err := store.RecentByAuthority(context.Background(), strconv.Itoa(a.AreaID), 200)
	if err != nil {
		t.Fatalf("RecentByAuthority: %v", err)
	}
	if len(got) != 1 || got[0].Name != a.Name {
		t.Fatalf("results: got %+v", got)
	}
	// Scoped to the authorityCode logical partition (never cross-partition).
	if items.lastQueryPK != strconv.Itoa(a.AreaID) {
		t.Errorf("query partition key: got %q, want %q", items.lastQueryPK, strconv.Itoa(a.AreaID))
	}
	// Bounded top-N ordered by the index-backed lastDifferent field, DESC. Must
	// NOT order by startDate (unindexed -> full-partition scan).
	want := "SELECT TOP @cap * FROM c ORDER BY c.lastDifferent DESC"
	if items.lastQuery != want {
		t.Errorf("recent query:\n got %q\nwant %q", items.lastQuery, want)
	}
	if strings.Contains(items.lastQuery, "startDate") {
		t.Errorf("query must not order by startDate (unindexed): %s", items.lastQuery)
	}
	// The cap is bound as a named parameter, not concatenated.
	if got, ok := items.lastQueryParams["@cap"]; !ok || got != 200 {
		t.Errorf("@cap param: got %v (present=%v), want 200", got, ok)
	}
}

func TestCosmosStore_RecentByAuthority_CapsAtCap(t *testing.T) {
	t.Parallel()
	const wantCap = 5
	items := newFakeItems()
	// A busy authority with more than wantCap documents in the partition.
	items.queryResult = recentDocs(t, wantCap+8)
	store := NewCosmosStore(items)

	got, err := store.RecentByAuthority(context.Background(), "471", wantCap)
	if err != nil {
		t.Fatalf("RecentByAuthority: %v", err)
	}
	if len(got) != wantCap {
		t.Errorf("busy authority: got %d results, want exactly cap=%d", len(got), wantCap)
	}
}

func TestCosmosStore_RecentByAuthority_EmptyResultIsEmptySlice(t *testing.T) {
	t.Parallel()
	store := NewCosmosStore(newFakeItems())

	got, err := store.RecentByAuthority(context.Background(), "471", 200)
	if err != nil {
		t.Fatalf("RecentByAuthority: %v", err)
	}
	if got == nil {
		t.Fatal("RecentByAuthority: got nil slice, want empty non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("RecentByAuthority: got %d results, want 0", len(got))
	}
}

func TestCosmosStore_RecentNearby_BoundedSpatialOrderedScopedToPartition(t *testing.T) {
	t.Parallel()
	a := testApplication(t)
	body, _ := json.Marshal(newApplicationDocument(a))
	items := newFakeItems()
	items.queryResult = [][]byte{body}
	store := NewCosmosStore(items)

	got, err := store.RecentNearby(context.Background(), "441", 51.4975, -0.1357, 5000, 200)
	if err != nil {
		t.Fatalf("RecentNearby: %v", err)
	}
	if len(got) != 1 || got[0].Name != a.Name {
		t.Fatalf("RecentNearby results: got %+v", got)
	}
	// Scoped to the authorityCode logical partition (never cross-partition).
	if items.lastQueryPK != "441" {
		t.Errorf("query partition key: got %q, want \"441\"", items.lastQueryPK)
	}
	// Bounded TOP @cap, single-partition spatial filter, ordered by the
	// index-backed lastDifferent field DESC. The GeoJSON point carries
	// [longitude, latitude] (GeoJSON order), mirroring findNearbyQuery. Must NOT
	// order by startDate (unindexed -> full-partition scan).
	want := "SELECT TOP @cap * FROM c WHERE ST_DISTANCE(c.location, " +
		`{"type": "Point", "coordinates": [@longitude, @latitude]}) <= @radiusMetres ` +
		"ORDER BY c.lastDifferent DESC"
	if items.lastQuery != want {
		t.Errorf("recent nearby query:\n got %q\nwant %q", items.lastQuery, want)
	}
	if strings.Contains(items.lastQuery, "startDate") {
		t.Errorf("query must not order by startDate (unindexed): %s", items.lastQuery)
	}
}

func TestCosmosStore_RecentNearby_UsesParameterizedQuery(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	store := NewCosmosStore(items)

	_, _ = store.RecentNearby(context.Background(), "441", 51.4975, -0.1357, 5000, 200)

	// No coordinate, radius, or cap value may be string-concatenated into the
	// query text — they must all be bound as named parameters.
	if items.lastQuery == "" {
		t.Fatal("no query was issued")
	}
	for _, literal := range []string{"51.4975", "-0.1357", "5000", "200"} {
		if strings.Contains(items.lastQuery, literal) {
			t.Errorf("query contains literal %q — should be a named parameter; query: %s", literal, items.lastQuery)
		}
	}
	wantParams := map[string]any{
		"@latitude":     51.4975,
		"@longitude":    -0.1357,
		"@radiusMetres": 5000.0,
		"@cap":          200,
	}
	for k, wantVal := range wantParams {
		gotVal, ok := items.lastQueryParams[k]
		if !ok {
			t.Errorf("param %q not found in query params %v", k, items.lastQueryParams)
			continue
		}
		if gotVal != wantVal {
			t.Errorf("param %q: got %v, want %v", k, gotVal, wantVal)
		}
	}
}

func TestCosmosStore_RecentNearby_CapsAtCap(t *testing.T) {
	t.Parallel()
	const wantCap = 5
	items := newFakeItems()
	// A busy authority with more than wantCap documents within the radius.
	items.queryResult = recentDocs(t, wantCap+8)
	store := NewCosmosStore(items)

	got, err := store.RecentNearby(context.Background(), "471", 51.5, -0.1, 5000, wantCap)
	if err != nil {
		t.Fatalf("RecentNearby: %v", err)
	}
	if len(got) != wantCap {
		t.Errorf("busy authority: got %d results, want exactly cap=%d", len(got), wantCap)
	}
}

func TestCosmosStore_RecentNearby_EmptyResultIsEmptySlice(t *testing.T) {
	t.Parallel()
	store := NewCosmosStore(newFakeItems())

	got, err := store.RecentNearby(context.Background(), "471", 51.5, -0.1, 5000, 200)
	if err != nil {
		t.Fatalf("RecentNearby: %v", err)
	}
	if got == nil {
		t.Fatal("RecentNearby: got nil slice, want empty non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("RecentNearby: got %d results, want 0", len(got))
	}
}

func TestCosmosStore_CountByAuthority_DecodesScalarScopedToPartition(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	// SELECT VALUE COUNT(1) returns a single row that is a bare JSON number.
	items.queryResult = [][]byte{[]byte("42")}
	store := NewCosmosStore(items)

	got, err := store.CountByAuthority(context.Background(), "471")
	if err != nil {
		t.Fatalf("CountByAuthority: %v", err)
	}
	if got != 42 {
		t.Errorf("count: got %d, want 42", got)
	}
	// Scoped to the authorityCode logical partition (never cross-partition).
	if items.lastQueryPK != "471" {
		t.Errorf("query partition key: got %q, want \"471\"", items.lastQueryPK)
	}
	// Index-only scalar count over the whole partition — no TOP @cap (that would
	// saturate the total), no ordering.
	want := "SELECT VALUE COUNT(1) FROM c"
	if items.lastQuery != want {
		t.Errorf("count query:\n got %q\nwant %q", items.lastQuery, want)
	}
	if _, ok := items.lastQueryParams["@cap"]; ok {
		t.Errorf("count query must not bind @cap (it counts the whole partition): %v", items.lastQueryParams)
	}
}

func TestCosmosStore_CountByAuthority_EmptyResultIsZero(t *testing.T) {
	t.Parallel()
	// No rows at all (defensive): an empty result set yields 0, not an error.
	store := NewCosmosStore(newFakeItems())

	got, err := store.CountByAuthority(context.Background(), "471")
	if err != nil {
		t.Fatalf("CountByAuthority: %v", err)
	}
	if got != 0 {
		t.Errorf("count: got %d, want 0", got)
	}
}

func TestCosmosStore_CountByAuthority_UsesLongReadBudget(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	items.queryResult = [][]byte{[]byte("3")}
	store := NewCosmosStore(items)

	if _, err := store.CountByAuthority(context.Background(), "156"); err != nil {
		t.Fatalf("CountByAuthority: %v", err)
	}
	if !items.lastQueryViaLongRead {
		t.Error("CountByAuthority must use the long-read budget (QueryItemsLongRead), not the 1.5s OLTP QueryItems")
	}
}

func TestCosmosStore_CountByAuthority_PropagatesQueryError(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	items.queryErr = context.DeadlineExceeded
	store := NewCosmosStore(items)

	if _, err := store.CountByAuthority(context.Background(), "471"); err == nil {
		t.Fatal("CountByAuthority: expected error, got nil")
	}
}

func TestCosmosStore_CountNearby_DecodesScalarWithSpatialFilterScopedToPartition(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	items.queryResult = [][]byte{[]byte("17")}
	store := NewCosmosStore(items)

	got, err := store.CountNearby(context.Background(), "441", 51.4975, -0.1357, 5000)
	if err != nil {
		t.Fatalf("CountNearby: %v", err)
	}
	if got != 17 {
		t.Errorf("count: got %d, want 17", got)
	}
	if items.lastQueryPK != "441" {
		t.Errorf("query partition key: got %q, want \"441\"", items.lastQueryPK)
	}
	// Same single-partition ST_DISTANCE filter and named-param style as
	// recentNearbyQuery, but a scalar COUNT instead of a bounded TOP @cap.
	want := "SELECT VALUE COUNT(1) FROM c WHERE ST_DISTANCE(c.location, " +
		`{"type": "Point", "coordinates": [@longitude, @latitude]}) <= @radiusMetres`
	if items.lastQuery != want {
		t.Errorf("count nearby query:\n got %q\nwant %q", items.lastQuery, want)
	}
	if _, ok := items.lastQueryParams["@cap"]; ok {
		t.Errorf("count query must not bind @cap: %v", items.lastQueryParams)
	}
}

func TestCosmosStore_CountNearby_UsesParameterizedQuery(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	store := NewCosmosStore(items)

	_, _ = store.CountNearby(context.Background(), "441", 51.4975, -0.1357, 5000)

	// No coordinate or radius value may be string-concatenated into the query —
	// they must all be bound as named parameters.
	if items.lastQuery == "" {
		t.Fatal("no query was issued")
	}
	for _, literal := range []string{"51.4975", "-0.1357", "5000"} {
		if strings.Contains(items.lastQuery, literal) {
			t.Errorf("query contains literal %q — should be a named parameter; query: %s", literal, items.lastQuery)
		}
	}
	wantParams := map[string]any{
		"@latitude":     51.4975,
		"@longitude":    -0.1357,
		"@radiusMetres": 5000.0,
	}
	for k, wantVal := range wantParams {
		gotVal, ok := items.lastQueryParams[k]
		if !ok {
			t.Errorf("param %q not found in query params %v", k, items.lastQueryParams)
			continue
		}
		if gotVal != wantVal {
			t.Errorf("param %q: got %v, want %v", k, gotVal, wantVal)
		}
	}
}

func TestCosmosStore_CountNearby_EmptyResultIsZero(t *testing.T) {
	t.Parallel()
	store := NewCosmosStore(newFakeItems())

	got, err := store.CountNearby(context.Background(), "471", 51.5, -0.1, 5000)
	if err != nil {
		t.Fatalf("CountNearby: %v", err)
	}
	if got != 0 {
		t.Errorf("count: got %d, want 0", got)
	}
}

func TestCosmosStore_CountNearby_UsesLongReadBudget(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	items.queryResult = [][]byte{[]byte("9")}
	store := NewCosmosStore(items)

	if _, err := store.CountNearby(context.Background(), "156", 51.5, -0.1, 5000); err != nil {
		t.Fatalf("CountNearby: %v", err)
	}
	if !items.lastQueryViaLongRead {
		t.Error("CountNearby must use the long-read budget (QueryItemsLongRead), not the 1.5s OLTP QueryItems")
	}
}

// The build-time SEO reads run over a LARGE authority partition and legitimately
// exceed the tight 1.5s OLTP budget, so they must route through the longer-budget
// QueryItemsLongRead accessor — never the OLTP QueryItems (tc-9tov).

func TestCosmosStore_RecentByAuthority_UsesLongReadBudget(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	store := NewCosmosStore(items)

	if _, err := store.RecentByAuthority(context.Background(), "156", 200); err != nil {
		t.Fatalf("RecentByAuthority: %v", err)
	}
	if !items.lastQueryViaLongRead {
		t.Error("RecentByAuthority must use the long-read budget (QueryItemsLongRead), not the 1.5s OLTP QueryItems")
	}
}

func TestCosmosStore_RecentNearby_UsesLongReadBudget(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	store := NewCosmosStore(items)

	if _, err := store.RecentNearby(context.Background(), "156", 51.5, -0.1, 5000, 200); err != nil {
		t.Fatalf("RecentNearby: %v", err)
	}
	if !items.lastQueryViaLongRead {
		t.Error("RecentNearby must use the long-read budget (QueryItemsLongRead), not the 1.5s OLTP QueryItems")
	}
}

// The user-facing reads must keep the tight OLTP budget — only the build-time SEO
// reads get the longer one, so widening must never leak onto the OLTP path.

func TestCosmosStore_FindNearby_KeepsOLTPBudget(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	store := NewCosmosStore(items)

	if _, err := store.FindNearby(context.Background(), "441", 51.4975, -0.1357, 2000); err != nil {
		t.Fatalf("FindNearby: %v", err)
	}
	if items.lastQueryViaLongRead {
		t.Error("FindNearby is a user-facing read and must keep the 1.5s OLTP QueryItems budget")
	}
}

func TestCosmosStore_GetByUID_KeepsOLTPBudget(t *testing.T) {
	t.Parallel()
	a := testApplication(t)
	body, _ := json.Marshal(newApplicationDocument(a))
	items := newFakeItems()
	items.queryResult = [][]byte{body}
	store := NewCosmosStore(items)

	if _, _, err := store.GetByUID(context.Background(), a.UID, strconv.Itoa(a.AreaID)); err != nil {
		t.Fatalf("GetByUID: %v", err)
	}
	if items.lastQueryViaLongRead {
		t.Error("GetByUID is a user-facing read and must keep the 1.5s OLTP QueryItems budget")
	}
}
