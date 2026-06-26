package applications

import (
	"context"
	"encoding/json"
	"errors"
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
	// pageResult, pageNext, pageErr and the lastPage* fields back
	// QueryPageCrossPartition, the bounded single-page fan-out FindNearbyPage uses
	// (tc-fm8f). A non-empty pageNext models "more pages remain".
	pageResult           [][]byte
	pageNext             string
	pageErr              error
	lastPageQuery        string
	lastPageParams       map[string]any
	lastPageSize         int
	lastPageContinuation string
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

func (f *fakeItems) QueryPageCrossPartition(_ context.Context, query string, params map[string]any, pageSize int, continuationToken string) ([][]byte, string, error) {
	f.lastPageQuery = query
	f.lastPageParams = params
	f.lastPageSize = pageSize
	f.lastPageContinuation = continuationToken
	if f.pageErr != nil {
		return nil, "", f.pageErr
	}
	return f.pageResult, f.pageNext, nil
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

func TestCosmosStore_FindNearbyPage_FetchesOneBoundedPageWithCursor(t *testing.T) {
	t.Parallel()
	// A border-spanning circle: two in-radius applications tagged to different
	// authority partitions (449 + 246). The bounded fan-out must surface BOTH and
	// must run via the single-page QueryPageCrossPartition, NOT the unbounded
	// QueryItemsCrossPartition drain (tc-fm8f).
	app449 := testApplication(t)
	app449.AreaID = 449
	app449.Name = "449/24/001"
	app246 := testApplication(t)
	app246.AreaID = 246
	app246.Name = "246/24/002"
	body449, _ := json.Marshal(newApplicationDocument(app449))
	body246, _ := json.Marshal(newApplicationDocument(app246))
	items := newFakeItems()
	items.pageResult = [][]byte{body449, body246}
	items.pageNext = "continuation-token-xyz"
	store := NewCosmosStore(items)

	got, next, err := store.FindNearbyPage(context.Background(), 51.4975, -0.1357, 2000, 500, "")
	if err != nil {
		t.Fatalf("FindNearbyPage: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("FindNearbyPage results: got %d, want 2 (both authorities); %+v", len(got), got)
	}
	gotAreas := map[int]bool{got[0].AreaID: true, got[1].AreaID: true}
	if !gotAreas[449] || !gotAreas[246] {
		t.Errorf("FindNearbyPage must surface both authorities 449 and 246; got %+v", gotAreas)
	}
	// The continuation token is passed straight back to the caller.
	if next != "continuation-token-xyz" {
		t.Errorf("next cursor: got %q, want %q", next, "continuation-token-xyz")
	}
	// The page size hint must be the requested limit so Cosmos caps the page at the
	// query layer — never an unbounded drain.
	if items.lastPageSize != 500 {
		t.Errorf("page size: got %d, want 500", items.lastPageSize)
	}
	// First page: no continuation token sent in.
	if items.lastPageContinuation != "" {
		t.Errorf("first page must not send a continuation token; got %q", items.lastPageContinuation)
	}
	// The constant-radius residual is preserved (same query as the unbounded form):
	// ST_DISTANCE(...) <= @radiusMetres, GeoJSON [longitude, latitude] order. No
	// ORDER BY — the Gateway rejects cross-partition ordering.
	want := "SELECT * FROM c WHERE ST_DISTANCE(c.location, " +
		`{"type": "Point", "coordinates": [@longitude, @latitude]}) <= @radiusMetres`
	if items.lastPageQuery != want {
		t.Errorf("bounded spatial query:\n got %q\nwant %q", items.lastPageQuery, want)
	}
}

func TestCosmosStore_FindNearbyPage_ResumesFromCursor(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	store := NewCosmosStore(items)

	if _, _, err := store.FindNearbyPage(context.Background(), 51.4975, -0.1357, 2000, 250, "resume-here"); err != nil {
		t.Fatalf("FindNearbyPage: %v", err)
	}
	if items.lastPageContinuation != "resume-here" {
		t.Errorf("continuation token: got %q, want %q", items.lastPageContinuation, "resume-here")
	}
	if items.lastPageSize != 250 {
		t.Errorf("page size: got %d, want 250", items.lastPageSize)
	}
}

func TestCosmosStore_FindNearbyPage_UsesParameterizedQuery(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	store := NewCosmosStore(items)

	_, _, _ = store.FindNearbyPage(context.Background(), 51.4975, -0.1357, 2000, 500, "")

	if items.lastPageQuery == "" {
		t.Fatal("no query was issued")
	}
	// The query text must not contain any float literal — values must be bound as
	// named parameters, not string-concatenated.
	for _, literal := range []string{"51.4975", "-0.1357", "2000"} {
		if strings.Contains(items.lastPageQuery, literal) {
			t.Errorf("query contains float literal %q — should be a named parameter; query: %s", literal, items.lastPageQuery)
		}
	}
	wantParams := map[string]any{
		"@latitude":     51.4975,
		"@longitude":    -0.1357,
		"@radiusMetres": 2000.0,
	}
	for k, wantVal := range wantParams {
		gotVal, ok := items.lastPageParams[k]
		if !ok {
			t.Errorf("param %q not found in query params %v", k, items.lastPageParams)
			continue
		}
		if gotVal != wantVal {
			t.Errorf("param %q: got %v, want %v", k, gotVal, wantVal)
		}
	}
}

func TestCosmosStore_FindNearbyPage_EmptyResultIsEmptySlice(t *testing.T) {
	t.Parallel()
	store := NewCosmosStore(newFakeItems())

	got, next, err := store.FindNearbyPage(context.Background(), 51.4975, -0.1357, 2000, 500, "")
	if err != nil {
		t.Fatalf("FindNearbyPage: %v", err)
	}
	if got == nil {
		t.Fatal("FindNearbyPage: got nil slice, want empty non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("FindNearbyPage: got %d results, want 0", len(got))
	}
	if next != "" {
		t.Errorf("exhausted query must return an empty cursor; got %q", next)
	}
}

func TestCosmosStore_FindNearbyPage_PropagatesError(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	items.pageErr = errors.New("gateway boom")
	store := NewCosmosStore(items)

	_, _, err := store.FindNearbyPage(context.Background(), 51.4975, -0.1357, 2000, 500, "")
	if err == nil {
		t.Fatal("FindNearbyPage: want error, got nil")
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

func TestCosmosStore_NearestNearby_BoundedSpatialDistanceOrderedScopedToPartition(t *testing.T) {
	t.Parallel()
	a := testApplication(t)
	body, _ := json.Marshal(newApplicationDocument(a))
	items := newFakeItems()
	items.queryResult = [][]byte{body}
	store := NewCosmosStore(items)

	got, err := store.NearestNearby(context.Background(), "441", 51.4975, -0.1357, 5000, 200)
	if err != nil {
		t.Fatalf("NearestNearby: %v", err)
	}
	if len(got) != 1 || got[0].Name != a.Name {
		t.Fatalf("NearestNearby results: got %+v", got)
	}
	// Scoped to the authorityCode logical partition (never cross-partition).
	if items.lastQueryPK != "441" {
		t.Errorf("query partition key: got %q, want \"441\"", items.lastQueryPK)
	}
	// Bounded TOP @cap, single-partition spatial filter, ordered by ST_DISTANCE
	// ASC (nearest first) — NOT by lastDifferent. The GeoJSON point carries
	// [longitude, latitude] (GeoJSON order), mirroring recentNearbyQuery. The
	// ORDER BY repeats the same parameterized ST_DISTANCE expression as the
	// WHERE filter.
	want := "SELECT TOP @cap * FROM c WHERE ST_DISTANCE(c.location, " +
		`{"type": "Point", "coordinates": [@longitude, @latitude]}) <= @radiusMetres ` +
		"ORDER BY ST_DISTANCE(c.location, " +
		`{"type": "Point", "coordinates": [@longitude, @latitude]})`
	if items.lastQuery != want {
		t.Errorf("nearest nearby query:\n got %q\nwant %q", items.lastQuery, want)
	}
	if strings.Contains(items.lastQuery, "lastDifferent") {
		t.Errorf("distance-ordered query must not order by lastDifferent: %s", items.lastQuery)
	}
}

func TestCosmosStore_NearestNearby_UsesParameterizedQuery(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	store := NewCosmosStore(items)

	_, _ = store.NearestNearby(context.Background(), "441", 51.4975, -0.1357, 5000, 200)

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

func TestCosmosStore_NearestNearby_CapsAtCap(t *testing.T) {
	t.Parallel()
	const wantCap = 5
	items := newFakeItems()
	// A busy authority with more than wantCap documents within the radius.
	items.queryResult = recentDocs(t, wantCap+8)
	store := NewCosmosStore(items)

	got, err := store.NearestNearby(context.Background(), "471", 51.5, -0.1, 5000, wantCap)
	if err != nil {
		t.Fatalf("NearestNearby: %v", err)
	}
	if len(got) != wantCap {
		t.Errorf("busy authority: got %d results, want exactly cap=%d", len(got), wantCap)
	}
}

func TestCosmosStore_NearestNearby_EmptyResultIsEmptySlice(t *testing.T) {
	t.Parallel()
	store := NewCosmosStore(newFakeItems())

	got, err := store.NearestNearby(context.Background(), "471", 51.5, -0.1, 5000, 200)
	if err != nil {
		t.Fatalf("NearestNearby: %v", err)
	}
	if got == nil {
		t.Fatal("NearestNearby: got nil slice, want empty non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("NearestNearby: got %d results, want 0", len(got))
	}
}

func TestCosmosStore_NearestNearby_UsesLongReadBudget(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	store := NewCosmosStore(items)

	if _, err := store.NearestNearby(context.Background(), "156", 51.5, -0.1, 5000, 200); err != nil {
		t.Fatalf("NearestNearby: %v", err)
	}
	if !items.lastQueryViaLongRead {
		t.Error("NearestNearby must use the long-read budget (QueryItemsLongRead), not the 1.5s OLTP QueryItems")
	}
}

// breakdownRow marshals a single GROUP BY projection row. A nil state omits the
// appState property entirely (Cosmos omits an undefined projection), modelling
// the "missing" case; a non-nil state emits an explicit string. The explicit
// JSON-null case is built inline by the test that needs it.
func breakdownRow(t *testing.T, state *string, count int) []byte {
	t.Helper()
	row := map[string]any{"count": count}
	if state != nil {
		row["appState"] = *state
	}
	body, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("marshal breakdown row: %v", err)
	}
	return body
}

func TestCosmosStore_BreakdownByAuthority_DecodesGroupByScopedToPartition(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	// GROUP BY returns one row per appState: {"appState": "...", "count": N}.
	items.queryResult = [][]byte{
		breakdownRow(t, strPtr("Permitted"), 2100),
		breakdownRow(t, strPtr("Rejected"), 900),
	}
	store := NewCosmosStore(items)

	got, err := store.BreakdownByAuthority(context.Background(), "471")
	if err != nil {
		t.Fatalf("BreakdownByAuthority: %v", err)
	}
	want := []StateCount{
		{AppState: strPtr("Permitted"), Count: 2100},
		{AppState: strPtr("Rejected"), Count: 900},
	}
	assertBreakdownEqual(t, got, want)
	// Scoped to the authorityCode logical partition (never cross-partition).
	if items.lastQueryPK != "471" {
		t.Errorf("query partition key: got %q, want \"471\"", items.lastQueryPK)
	}
	// Index-served GROUP BY over the whole partition — no TOP @cap (that would
	// saturate the total).
	wantQuery := "SELECT c.appState, COUNT(1) AS count FROM c GROUP BY c.appState"
	if items.lastQuery != wantQuery {
		t.Errorf("breakdown query:\n got %q\nwant %q", items.lastQuery, wantQuery)
	}
	if _, ok := items.lastQueryParams["@cap"]; ok {
		t.Errorf("breakdown query must not bind @cap (it spans the whole partition): %v", items.lastQueryParams)
	}
}

func TestCosmosStore_BreakdownByAuthority_OrdersCountDescStateAscNilLast(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	// Deliberately unsorted, with a tie (Permitted vs Rejected both 5) and a nil
	// bucket tying on count too, to prove the deterministic ordering.
	items.queryResult = [][]byte{
		breakdownRow(t, strPtr("Rejected"), 5),
		breakdownRow(t, nil, 5),
		breakdownRow(t, strPtr("Conditions"), 9),
		breakdownRow(t, strPtr("Permitted"), 5),
	}
	store := NewCosmosStore(items)

	got, err := store.BreakdownByAuthority(context.Background(), "471")
	if err != nil {
		t.Fatalf("BreakdownByAuthority: %v", err)
	}
	// count DESC primary; on the 5-way tie, appState ASC then nil last.
	want := []StateCount{
		{AppState: strPtr("Conditions"), Count: 9},
		{AppState: strPtr("Permitted"), Count: 5},
		{AppState: strPtr("Rejected"), Count: 5},
		{AppState: nil, Count: 5},
	}
	assertBreakdownEqual(t, got, want)
}

func TestCosmosStore_BreakdownByAuthority_MissingAndNullAppStateFoldIntoOneNilBucket(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	// One row omits appState (absent projection); a separate row carries explicit
	// JSON null. Both must merge into the SINGLE nil bucket, matching
	// BreakdownNearby's nil semantics.
	items.queryResult = [][]byte{
		breakdownRow(t, nil, 30),                  // appState absent
		[]byte(`{"appState": null, "count": 12}`), // appState explicit null
		breakdownRow(t, strPtr("Permitted"), 100),
	}
	store := NewCosmosStore(items)

	got, err := store.BreakdownByAuthority(context.Background(), "471")
	if err != nil {
		t.Fatalf("BreakdownByAuthority: %v", err)
	}
	// 30 + 12 = 42 fold into one nil bucket; Permitted (100) outranks it.
	want := []StateCount{
		{AppState: strPtr("Permitted"), Count: 100},
		{AppState: nil, Count: 42},
	}
	assertBreakdownEqual(t, got, want)
}

func TestCosmosStore_BreakdownByAuthority_EmptyPartitionIsEmptySlice(t *testing.T) {
	t.Parallel()
	// No rows at all (empty partition): an empty result set yields an empty
	// non-nil slice, not an error.
	store := NewCosmosStore(newFakeItems())

	got, err := store.BreakdownByAuthority(context.Background(), "471")
	if err != nil {
		t.Fatalf("BreakdownByAuthority: %v", err)
	}
	if got == nil {
		t.Fatal("BreakdownByAuthority: got nil slice, want empty non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("BreakdownByAuthority: got %d entries, want 0", len(got))
	}
}

func TestCosmosStore_BreakdownByAuthority_UsesLongReadBudget(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	items.queryResult = [][]byte{breakdownRow(t, strPtr("Permitted"), 3)}
	store := NewCosmosStore(items)

	if _, err := store.BreakdownByAuthority(context.Background(), "156"); err != nil {
		t.Fatalf("BreakdownByAuthority: %v", err)
	}
	if !items.lastQueryViaLongRead {
		t.Error("BreakdownByAuthority must use the long-read budget (QueryItemsLongRead), not the 1.5s OLTP QueryItems")
	}
}

func TestCosmosStore_BreakdownByAuthority_PropagatesQueryError(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	items.queryErr = context.DeadlineExceeded
	store := NewCosmosStore(items)

	if _, err := store.BreakdownByAuthority(context.Background(), "471"); err == nil {
		t.Fatal("BreakdownByAuthority: expected error, got nil")
	}
}

func TestCosmosStore_BreakdownNearby_DecodesGroupByWithSpatialFilterScopedToPartition(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	// GROUP BY returns one row per appState over the whole in-radius set:
	// {"appState": "...", "count": N}.
	items.queryResult = [][]byte{
		breakdownRow(t, strPtr("Permitted"), 2100),
		breakdownRow(t, strPtr("Rejected"), 900),
	}
	store := NewCosmosStore(items)

	got, err := store.BreakdownNearby(context.Background(), "441", 51.4975, -0.1357, 5000)
	if err != nil {
		t.Fatalf("BreakdownNearby: %v", err)
	}
	want := []StateCount{
		{AppState: strPtr("Permitted"), Count: 2100},
		{AppState: strPtr("Rejected"), Count: 900},
	}
	assertBreakdownEqual(t, got, want)
	// Scoped to the authorityCode logical partition (never cross-partition).
	if items.lastQueryPK != "441" {
		t.Errorf("query partition key: got %q, want \"441\"", items.lastQueryPK)
	}
	// Same single-partition ST_DISTANCE filter and named-param style as
	// recentNearbyQuery, married to an index-served GROUP BY over the whole
	// in-radius set — no TOP @cap (that would saturate the total).
	wantQuery := "SELECT c.appState, COUNT(1) AS count FROM c WHERE ST_DISTANCE(c.location, " +
		`{"type": "Point", "coordinates": [@longitude, @latitude]}) <= @radiusMetres ` +
		"GROUP BY c.appState"
	if items.lastQuery != wantQuery {
		t.Errorf("breakdown nearby query:\n got %q\nwant %q", items.lastQuery, wantQuery)
	}
	if _, ok := items.lastQueryParams["@cap"]; ok {
		t.Errorf("breakdown query must not bind @cap (it spans the whole in-radius set): %v", items.lastQueryParams)
	}
}

func TestCosmosStore_BreakdownNearby_UsesParameterizedQuery(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	store := NewCosmosStore(items)

	_, _ = store.BreakdownNearby(context.Background(), "441", 51.4975, -0.1357, 5000)

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

func TestCosmosStore_BreakdownNearby_OrdersCountDescStateAscNilLast(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	// Deliberately unsorted, with a tie (Permitted vs Rejected both 5) and a nil
	// bucket tying on count too, to prove the deterministic ordering.
	items.queryResult = [][]byte{
		breakdownRow(t, strPtr("Rejected"), 5),
		breakdownRow(t, nil, 5),
		breakdownRow(t, strPtr("Conditions"), 9),
		breakdownRow(t, strPtr("Permitted"), 5),
	}
	store := NewCosmosStore(items)

	got, err := store.BreakdownNearby(context.Background(), "471", 51.5, -0.1, 5000)
	if err != nil {
		t.Fatalf("BreakdownNearby: %v", err)
	}
	// count DESC primary; on the 5-way tie, appState ASC then nil last.
	want := []StateCount{
		{AppState: strPtr("Conditions"), Count: 9},
		{AppState: strPtr("Permitted"), Count: 5},
		{AppState: strPtr("Rejected"), Count: 5},
		{AppState: nil, Count: 5},
	}
	assertBreakdownEqual(t, got, want)
}

func TestCosmosStore_BreakdownNearby_MissingAndNullAppStateFoldIntoOneNilBucket(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	// One row omits appState (absent projection); a separate row carries explicit
	// JSON null. Both must merge into the SINGLE nil bucket, matching
	// BreakdownByAuthority's nil semantics.
	items.queryResult = [][]byte{
		breakdownRow(t, nil, 30),                  // appState absent
		[]byte(`{"appState": null, "count": 12}`), // appState explicit null
		breakdownRow(t, strPtr("Permitted"), 100),
	}
	store := NewCosmosStore(items)

	got, err := store.BreakdownNearby(context.Background(), "471", 51.5, -0.1, 5000)
	if err != nil {
		t.Fatalf("BreakdownNearby: %v", err)
	}
	// 30 + 12 = 42 fold into one nil bucket; Permitted (100) outranks it.
	want := []StateCount{
		{AppState: strPtr("Permitted"), Count: 100},
		{AppState: nil, Count: 42},
	}
	assertBreakdownEqual(t, got, want)
}

func TestCosmosStore_BreakdownNearby_EmptyResultIsEmptySlice(t *testing.T) {
	t.Parallel()
	// No rows at all (nothing in radius): an empty result set yields an empty
	// non-nil slice, not an error.
	store := NewCosmosStore(newFakeItems())

	got, err := store.BreakdownNearby(context.Background(), "471", 51.5, -0.1, 5000)
	if err != nil {
		t.Fatalf("BreakdownNearby: %v", err)
	}
	if got == nil {
		t.Fatal("BreakdownNearby: got nil slice, want empty non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("BreakdownNearby: got %d entries, want 0", len(got))
	}
}

func TestCosmosStore_BreakdownNearby_UsesLongReadBudget(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	items.queryResult = [][]byte{breakdownRow(t, strPtr("Permitted"), 3)}
	store := NewCosmosStore(items)

	if _, err := store.BreakdownNearby(context.Background(), "156", 51.5, -0.1, 5000); err != nil {
		t.Fatalf("BreakdownNearby: %v", err)
	}
	if !items.lastQueryViaLongRead {
		t.Error("BreakdownNearby must use the long-read budget (QueryItemsLongRead), not the 1.5s OLTP QueryItems")
	}
}

func TestCosmosStore_BreakdownNearby_PropagatesQueryError(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	items.queryErr = context.DeadlineExceeded
	store := NewCosmosStore(items)

	if _, err := store.BreakdownNearby(context.Background(), "471", 51.5, -0.1, 5000); err == nil {
		t.Fatal("BreakdownNearby: expected error, got nil")
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

func TestCosmosStore_FindNearbyPage_KeepsOLTPBudget(t *testing.T) {
	t.Parallel()
	items := newFakeItems()
	store := NewCosmosStore(items)

	if _, _, err := store.FindNearbyPage(context.Background(), 51.4975, -0.1357, 2000, 500, ""); err != nil {
		t.Fatalf("FindNearbyPage: %v", err)
	}
	// FindNearbyPage is a user-facing read: it fans out cross-partition but must
	// never route through the latency-tolerant build-time SEO budget
	// (QueryItemsLongRead).
	if items.lastPageQuery == "" {
		t.Error("FindNearbyPage must run the bounded cross-partition spatial query")
	}
	if items.lastQueryViaLongRead {
		t.Error("FindNearbyPage is a user-facing read and must not use the build-time long-read budget")
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
