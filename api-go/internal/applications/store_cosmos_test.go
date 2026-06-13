package applications

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

type fakeItems struct {
	stored       map[string][]byte // id -> bytes
	readErr      error
	upsertErr    error
	queryErr     error
	queryResult  [][]byte
	lastUpsertPK string
	lastUpsert   []byte
	lastReadPK   string
	lastReadID   string
	lastQueryPK  string
	lastQuery    string
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

func (f *fakeItems) QueryItems(_ context.Context, partitionKey, query string, _ map[string]any) ([][]byte, error) {
	f.lastQueryPK = partitionKey
	f.lastQuery = query
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
	// radius is the bare metres value, mirroring .NET's interpolated ST_DISTANCE.
	want := `SELECT * FROM c WHERE ST_DISTANCE(c.location, {"type": "Point", "coordinates": [-0.1357, 51.4975]}) <= 2000`
	if items.lastQuery != want {
		t.Errorf("spatial query:\n got %q\nwant %q", items.lastQuery, want)
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
