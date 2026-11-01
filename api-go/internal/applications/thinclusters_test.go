package applications

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"testing"
)

var applicationsTable = regexp.MustCompile(`\bapplications\b`)

func TestThinClusterQueries_ReadOnlyTheThinTable(t *testing.T) {
	t.Parallel()
	for name, q := range map[string]string{
		"zone all":        thinClusterQuery(clusterQueryAllStatuses),
		"zone status":     thinClusterQuery(clusterQueryByStatus),
		"boundary all":    thinClusterQuery(boundaryClusterQueryAllStatuses),
		"boundary status": thinClusterQuery(boundaryClusterQueryByStatus),
	} {
		if applicationsTable.MatchString(q) || strings.Contains(q, "FROM applications") {
			t.Errorf("%s references the applications table:\n%s", name, q)
		}
		if got := strings.Count(q, "FROM "+thinTable+"\n"); got != 1 {
			t.Errorf("%s reads %s %d times, want 1:\n%s", name, thinTable, got, q)
		}
	}
}

func TestClusterQueries_OffPathTextIsUnchanged(t *testing.T) {
	t.Parallel()
	want := map[string]string{
		"zone all":        "",
		"zone status":     "",
		"boundary all":    "",
		"boundary status": "",
	}
	got := map[string]string{
		"zone all":        clusterQueryAllStatuses,
		"zone status":     clusterQueryByStatus,
		"boundary all":    boundaryClusterQueryAllStatuses,
		"boundary status": boundaryClusterQueryByStatus,
	}
	for name, q := range got {
		sum := sha256.Sum256([]byte(q))
		if h := hex.EncodeToString(sum[:]); h != want[name] {
			t.Errorf("%s: sha256 = %s, want %s", name, h, want[name])
		}
	}
}

type fakeReadPath struct{ list, mapOn bool }

func (f fakeReadPath) ThinList(context.Context) bool { return f.list }
func (f fakeReadPath) ThinMap(context.Context) bool  { return f.mapOn }

func TestPostgresStore_ThinMapSwitch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if NewPostgresStore(nil).thinMap(ctx) {
		t.Error("thinMap() = true with no switch, want off")
	}
	if NewPostgresStore(nil).WithReadPath(fakeReadPath{list: true}).thinMap(ctx) {
		t.Error("thinMap() = true with only thin_list on")
	}
	if !NewPostgresStore(nil).WithReadPath(fakeReadPath{mapOn: true}).thinMap(ctx) {
		t.Error("thinMap() = false with thin_map on")
	}
}
