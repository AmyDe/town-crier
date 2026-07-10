package applications

import (
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// fakeClusterRow is a hand-written pgx.CollectableRow that feeds scanClusterRow a
// fixed set of column values in the exact order the SELECT projects them
// (centroid_lat, centroid_lon, member_count, status_counts, member_authority,
// member_name, members). It lets us cover the raw-JSONB decode of the new member
// list without a live database.
type fakeClusterRow struct {
	lat, lon   float64
	count      int64
	rawCounts  []byte
	authority  string
	name       string
	rawMembers []byte // nil models a SQL NULL members column
}

func (r fakeClusterRow) Scan(dest ...any) error {
	*(dest[0].(*float64)) = r.lat
	*(dest[1].(*float64)) = r.lon
	*(dest[2].(*int64)) = r.count
	*(dest[3].(*[]byte)) = r.rawCounts
	*(dest[4].(*string)) = r.authority
	*(dest[5].(*string)) = r.name
	*(dest[6].(*[]byte)) = r.rawMembers
	return nil
}

func (r fakeClusterRow) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r fakeClusterRow) Values() ([]any, error)                       { return nil, nil }
func (r fakeClusterRow) RawValues() [][]byte                          { return nil }

// TestScanClusterRow_Members proves scanClusterRow decodes a present applicationIds
// JSONB column into Cluster.Members, and leaves Members nil for a SQL NULL column.
func TestScanClusterRow_Members(t *testing.T) {
	t.Parallel()

	t.Run("present member list decodes into Members", func(t *testing.T) {
		t.Parallel()
		row := fakeClusterRow{
			lat: 51.5, lon: -0.12, count: 3,
			rawCounts:  []byte(`{"Permitted":3}`),
			authority:  "100",
			name:       "A1",
			rawMembers: []byte(`[{"authority":"100","name":"A1"},{"authority":"100","name":"A2"},{"authority":"100","name":"A3"}]`),
		}
		c, err := scanClusterRow(row)
		if err != nil {
			t.Fatalf("scanClusterRow: %v", err)
		}
		if len(c.Members) != 3 {
			t.Fatalf("Members: got %d, want 3 (%+v)", len(c.Members), c.Members)
		}
		want := []PlanningApplicationID{
			{Authority: "100", Name: "A1"},
			{Authority: "100", Name: "A2"},
			{Authority: "100", Name: "A3"},
		}
		for i, w := range want {
			if c.Members[i] != w {
				t.Errorf("Members[%d]: got %+v, want %+v", i, c.Members[i], w)
			}
		}
		// A multi-member cell still leaves the single-member id nil.
		if c.Member != nil {
			t.Errorf("Member: got %+v, want nil for a multi-member cell", c.Member)
		}
	})

	t.Run("NULL member column yields nil Members", func(t *testing.T) {
		t.Parallel()
		row := fakeClusterRow{
			lat: 51.5, lon: -0.12, count: 1,
			rawCounts:  []byte(`{"Permitted":1}`),
			authority:  "471",
			name:       "24/001",
			rawMembers: nil, // SQL NULL
		}
		c, err := scanClusterRow(row)
		if err != nil {
			t.Fatalf("scanClusterRow: %v", err)
		}
		if c.Members != nil {
			t.Errorf("Members: got %+v, want nil for a NULL members column", c.Members)
		}
		// A single-member cell still carries its applicationId.
		if c.Member == nil || c.Member.Authority != "471" || c.Member.Name != "24/001" {
			t.Errorf("Member: got %+v, want {471, 24/001}", c.Member)
		}
	})
}

// TestPlanningApplicationID_AuthoritySlugJSONShape pins the wire contract for
// the GH#924 AuthoritySlug addition: omitted when empty (byte-identical to the
// pre-GH#924 authed zone-clusters response, which never populates it) and
// present when the anonymous handler has set it.
func TestPlanningApplicationID_AuthoritySlugJSONShape(t *testing.T) {
	t.Parallel()

	t.Run("empty slug is omitted", func(t *testing.T) {
		t.Parallel()
		id := PlanningApplicationID{Authority: "100", Name: "24/001"}
		raw, err := json.Marshal(id)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var got map[string]json.RawMessage
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("unmarshal: %v; raw=%s", err, raw)
		}
		if _, ok := got["authoritySlug"]; ok {
			t.Errorf("authoritySlug must be omitted when empty, got %s", raw)
		}
	})

	t.Run("present slug is serialised", func(t *testing.T) {
		t.Parallel()
		id := PlanningApplicationID{Authority: "100", Name: "24/001", AuthoritySlug: "testshire"}
		raw, err := json.Marshal(id)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var got map[string]json.RawMessage
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("unmarshal: %v; raw=%s", err, raw)
		}
		slug, ok := got["authoritySlug"]
		if !ok {
			t.Fatalf("authoritySlug missing from %s", raw)
		}
		var s string
		if err := json.Unmarshal(slug, &s); err != nil || s != "testshire" {
			t.Errorf("authoritySlug: got %s, want \"testshire\"", slug)
		}
	})
}

// TestCluster_MembersJSONShape pins the wire contract for the new member list:
// applicationIds is present (an array) when Members is populated, and is omitted
// entirely (omitempty) for a splittable cell whose Members is nil.
func TestCluster_MembersJSONShape(t *testing.T) {
	t.Parallel()

	t.Run("coincident cell serialises applicationIds", func(t *testing.T) {
		t.Parallel()
		c := Cluster{
			Latitude: 51.5, Longitude: -0.12, Count: 2,
			StatusCounts: map[string]int{"Permitted": 2},
			Members: []PlanningApplicationID{
				{Authority: "100", Name: "A1"},
				{Authority: "100", Name: "A2"},
			},
		}
		raw, err := json.Marshal(c)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var got map[string]json.RawMessage
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("unmarshal: %v; raw=%s", err, raw)
		}
		ids, ok := got["applicationIds"]
		if !ok {
			t.Fatalf("applicationIds missing from %s", raw)
		}
		var members []PlanningApplicationID
		if err := json.Unmarshal(ids, &members); err != nil {
			t.Fatalf("decode applicationIds: %v", err)
		}
		if len(members) != 2 || members[0] != (PlanningApplicationID{Authority: "100", Name: "A1"}) {
			t.Errorf("applicationIds: got %+v, want the two seeded members", members)
		}
	})

	t.Run("splittable cell omits applicationIds", func(t *testing.T) {
		t.Parallel()
		c := Cluster{
			Latitude: 51.5, Longitude: -0.12, Count: 194,
			StatusCounts: map[string]int{"Permitted": 194},
			Members:      nil,
		}
		raw, err := json.Marshal(c)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var got map[string]json.RawMessage
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("unmarshal: %v; raw=%s", err, raw)
		}
		if _, ok := got["applicationIds"]; ok {
			t.Errorf("applicationIds must be omitted for a splittable cell, got %s", raw)
		}
	})
}
