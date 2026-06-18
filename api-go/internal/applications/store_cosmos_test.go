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
	f.lastQueryPK = partitionKey
	f.lastQuery = query
	f.lastQueryParams = params
	if f.queryErr != nil {
		return nil, f.queryErr
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
