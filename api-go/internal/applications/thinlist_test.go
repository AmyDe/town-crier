package applications

import (
	"strings"
	"testing"
)

func TestThinQuery_RewritesOntoTheThinTableOnly(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		sort  Sort
		query string
	}{
		{"newest first", SortNewest, newestFirstQuery},
		{"oldest keyset", SortOldest, oldestKeysetQuery},
		{"status keyset", SortStatus, statusKeysetQuery},
		{"activity first", SortRecentActivity, recentActivityFirstQuery},
		{"activity keyset", SortRecentActivity, recentActivityKeysetQuery},
		{"boundary newest first", SortNewest, boundaryNewestFirstQuery},
		{"boundary status tail", SortStatus, boundaryStatusKeysetBothNullQuery},
		{"boundary activity keyset", SortRecentActivity, boundaryRecentActivityKeysetQuery},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := thinQuery(tc.sort, tc.query)
			for _, banned := range []string{" FROM applications", "applications.", "description", "address"} {
				if strings.Contains(got, banned) {
					t.Errorf("thin query contains %q:\n%s", banned, got)
				}
			}
			if !strings.Contains(got, " FROM "+thinTable) {
				t.Errorf("thin query does not read %s:\n%s", thinTable, got)
			}
		})
	}
}

func TestThinQueries_NeverReadApplications(t *testing.T) {
	t.Parallel()
	for name, q := range map[string]string{
		"nearby first":    thinNearbyFirstQuery,
		"nearby keyset":   thinNearbyKeysetQuery,
		"boundary first":  thinBoundaryFirstQuery,
		"boundary keyset": thinBoundaryKeysetQuery,
	} {
		if strings.Contains(q, "FROM applications") {
			t.Errorf("%s reads applications:\n%s", name, q)
		}
	}
}

func TestBuildThinFilteredZoneQuery_ReadsOnlyTheThinTable(t *testing.T) {
	t.Parallel()
	for _, srt := range []Sort{SortDistance, SortNewest, SortOldest, SortStatus, SortRecentActivity} {
		q := InZoneQuery{UserID: "u", RadiusMetres: 1000, Sort: srt, Unread: true, Limit: 5}
		query, _, err := buildThinFilteredZoneQuery(q, nil)
		if err != nil {
			t.Fatalf("%s: %v", srt, err)
		}
		if strings.Contains(query, "applications") || !strings.Contains(query, thinTable+".uid") {
			t.Errorf("%s: query = %s", srt, query)
		}
	}
}

func TestBuildFilteredZoneQuery_OffPathTextIsUnchanged(t *testing.T) {
	t.Parallel()
	q := InZoneQuery{UserID: "u", RadiusMetres: 1000, Sort: SortRecentActivity, Unread: true, Limit: 5}
	got, _, err := buildFilteredZoneQuery(q, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "SELECT " + dateColumns + ", " + activityExpr + " AS activity_ts FROM applications" +
		" JOIN (SELECT n.application_uid, n.authority_id, MAX(n.created_at) AS created_at FROM notifications n" +
		" WHERE n.user_id = $3 AND n.read_at IS NULL GROUP BY n.application_uid, n.authority_id) u" +
		" ON applications.uid = u.application_uid AND applications.area_id = u.authority_id" +
		" WHERE ST_DWithin(location, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $4)" +
		" ORDER BY " + activityExpr + " DESC NULLS LAST, authority_code, planit_name LIMIT $5"
	if got != want {
		t.Errorf("query changed:\n got  %s\n want %s", got, want)
	}
}
