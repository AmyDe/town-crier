package applications

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSnapshotDocument_RoundTrip(t *testing.T) {
	t.Parallel()
	a := testApplication(t)
	got := NewSnapshotDocument(a).ToDomain()

	if got.Name != a.Name || got.UID != a.UID || got.AreaID != a.AreaID {
		t.Errorf("identity mismatch: %+v", got)
	}
	if !got.StartDate.Equal(*a.StartDate) || !got.LastDifferent.Equal(a.LastDifferent) {
		t.Errorf("date mismatch: %+v", got)
	}
	if *got.Longitude != *a.Longitude || *got.Latitude != *a.Latitude {
		t.Errorf("coords mismatch")
	}
}

func TestSnapshotDocument_KeysOmitContainerIdentity(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(NewSnapshotDocument(testApplication(t)))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)
	// The snapshot uses "name" and carries no master-doc identity keys.
	if !strings.Contains(body, `"name":"24/0123/FUL"`) {
		t.Errorf("missing name key: %s", body)
	}
	for _, absent := range []string{`"id":`, `"authorityCode":`, `"planitName":`} {
		if strings.Contains(body, absent) {
			t.Errorf("snapshot must not carry %s: %s", absent, body)
		}
	}
	if !strings.Contains(body, `"type":"Point"`) {
		t.Errorf("snapshot missing geojson location: %s", body)
	}
}
