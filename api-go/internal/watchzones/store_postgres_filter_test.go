package watchzones

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// fakeZoneRow is a hand-written pgx.Row test double scanning the 11-column
// pgZoneColumns projection scanZone expects: id, user_id, name, latitude,
// longitude, radius_metres, created_at, push_enabled, email_instant_enabled,
// boundary geojson, filter_key (GH#1090, epic tc-w825j). It exists purely to
// prove filter_key's round trip through scanZone at the fake level -- the
// store's broader read/write behaviour (list/get/delete/find, boundary
// geometry) is already covered by the real-DB integration suite in
// store_postgres_test.go.
type fakeZoneRow struct {
	id, userID, name                  string
	latitude, longitude, radiusMetres float64
	createdAt                         time.Time
	pushEnabled, emailFlag            bool
	boundaryGeoJSON                   *string
	filterKey                         *string
}

func (r *fakeZoneRow) Scan(dest ...any) error {
	if len(dest) != 11 {
		return errFakeZoneRowArity
	}
	*dest[0].(*string) = r.id
	*dest[1].(*string) = r.userID
	*dest[2].(*string) = r.name
	*dest[3].(*float64) = r.latitude
	*dest[4].(*float64) = r.longitude
	*dest[5].(*float64) = r.radiusMetres
	*dest[6].(*time.Time) = r.createdAt
	*dest[7].(*bool) = r.pushEnabled
	*dest[8].(*bool) = r.emailFlag
	*dest[9].(**string) = r.boundaryGeoJSON
	*dest[10].(**string) = r.filterKey
	return nil
}

var errFakeZoneRowArity = errors.New("fakeZoneRow: unexpected scan destination count")

// fakeZoneQuerier is a hand-written test double for the store's consumer-side
// querier interface. It exists only to drive the filter_key round-trip tests
// below: QueryRow always returns the configured row, and Exec records the
// bound args (and returns the configured error) so Save's filter_key
// parameter can be asserted without a live database.
type fakeZoneQuerier struct {
	row     pgx.Row
	execErr error
	gotArgs []any
}

func (f *fakeZoneQuerier) Exec(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
	f.gotArgs = args
	return pgconn.CommandTag{}, f.execErr
}

func (f *fakeZoneQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, errors.New("fakeZoneQuerier: Query not implemented")
}

func (f *fakeZoneQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return f.row
}

// TestPostgresStore_Get_FilterKeyPresent proves a non-NULL filter_key column
// hydrates onto WatchZone.FilterKey.
func TestPostgresStore_Get_FilterKeyPresent(t *testing.T) {
	t.Parallel()
	key := "house_builder"
	q := &fakeZoneQuerier{row: &fakeZoneRow{
		id: "zone-1", userID: "user-1", name: "Home",
		latitude: 51.5, longitude: -0.1, radiusMetres: 1000,
		createdAt: time.Now(), pushEnabled: true, emailFlag: true,
		filterKey: &key,
	}}
	store := NewPostgresStore(q)

	got, err := store.Get(context.Background(), "user-1", "zone-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.FilterKey != FilterKeyHouseBuilder {
		t.Errorf("FilterKey: got %q, want %q", got.FilterKey, FilterKeyHouseBuilder)
	}
}

// TestPostgresStore_Get_FilterKeyNullCoalescesToEmpty proves a NULL
// filter_key column (every zone before this migration, or one never
// filtered) hydrates as the zero value "" (unfiltered), not a nil-pointer
// panic.
func TestPostgresStore_Get_FilterKeyNullCoalescesToEmpty(t *testing.T) {
	t.Parallel()
	q := &fakeZoneQuerier{row: &fakeZoneRow{
		id: "zone-1", userID: "user-1", name: "Home",
		latitude: 51.5, longitude: -0.1, radiusMetres: 1000,
		createdAt: time.Now(), pushEnabled: true, emailFlag: true,
		filterKey: nil,
	}}
	store := NewPostgresStore(q)

	got, err := store.Get(context.Background(), "user-1", "zone-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.FilterKey != "" {
		t.Errorf("FilterKey: got %q, want empty (unfiltered)", got.FilterKey)
	}
}

// TestPostgresStore_Save_PassesFilterKeyArg proves Save binds a non-empty
// FilterKey as a pointer-to-string argument -- the last positional arg of
// pgSaveZoneQuery.
func TestPostgresStore_Save_PassesFilterKeyArg(t *testing.T) {
	t.Parallel()
	q := &fakeZoneQuerier{}
	store := NewPostgresStore(q)
	z := testZone(t)
	z.FilterKey = FilterKeyLoftExtension

	if err := store.Save(context.Background(), z); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if len(q.gotArgs) == 0 {
		t.Fatal("Save must bind args")
	}
	last := q.gotArgs[len(q.gotArgs)-1]
	s, ok := last.(*string)
	if !ok || s == nil || *s != string(FilterKeyLoftExtension) {
		t.Errorf("filter_key arg: got %#v, want pointer to %q", last, FilterKeyLoftExtension)
	}
}

// TestPostgresStore_Save_EmptyFilterKeyPassesNil proves an unfiltered zone
// (the FilterKey zero value) binds a nil argument -- SQL NULL -- for
// filter_key, mirroring how an unset Boundary binds nil.
func TestPostgresStore_Save_EmptyFilterKeyPassesNil(t *testing.T) {
	t.Parallel()
	q := &fakeZoneQuerier{}
	store := NewPostgresStore(q)
	z := testZone(t) // FilterKey left at its zero value ("")

	if err := store.Save(context.Background(), z); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if len(q.gotArgs) == 0 {
		t.Fatal("Save must bind args")
	}
	last := q.gotArgs[len(q.gotArgs)-1]
	if s, ok := last.(*string); !ok || s != nil {
		t.Errorf("filter_key arg for unfiltered zone: got %#v, want nil *string", last)
	}
}
