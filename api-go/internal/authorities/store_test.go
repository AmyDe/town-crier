package authorities

import (
	"encoding/json"
	"os"
	"testing"
)

func TestStaticStore_AllSortedByNameOrdinalIgnoreCase(t *testing.T) {
	t.Parallel()

	s := newStaticStore()
	all := s.all()

	// Golden order captured verbatim from the .NET dev API's GET /v1/authorities
	// response, which sorts by name with StringComparer.OrdinalIgnoreCase. The
	// Go store must reproduce the exact same ordering.
	raw, err := os.ReadFile("testdata/golden_authorities.json")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var golden struct {
		Authorities []struct {
			ID       int    `json:"id"`
			Name     string `json:"name"`
			AreaType string `json:"areaType"`
		} `json:"authorities"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatalf("unmarshal golden: %v", err)
	}

	if len(all) != len(golden.Authorities) {
		t.Fatalf("count: got %d, want %d", len(all), len(golden.Authorities))
	}
	if len(all) != golden.Total {
		t.Fatalf("count vs total: got %d, want %d", len(all), golden.Total)
	}
	for i := range all {
		w := golden.Authorities[i]
		if all[i].ID != w.ID || all[i].Name != w.Name || all[i].AreaType != w.AreaType {
			t.Errorf("position %d: got {%d %q %q}, want {%d %q %q}",
				i, all[i].ID, all[i].Name, all[i].AreaType, w.ID, w.Name, w.AreaType)
		}
	}
}

func TestStaticStore_ByID(t *testing.T) {
	t.Parallel()

	s := newStaticStore()

	tests := []struct {
		name     string
		id       int
		wantOK   bool
		wantName string
	}{
		{"existing id", 384, true, "Aberdeen"},
		{"another existing id", 245, true, "Adur"},
		{"missing id", 99999999, false, ""},
		{"zero id", 0, false, ""},
		{"negative id", -1, false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := s.byID(tc.id)
			if ok != tc.wantOK {
				t.Fatalf("ok: got %v, want %v", ok, tc.wantOK)
			}
			if ok && got.Name != tc.wantName {
				t.Errorf("name: got %q, want %q", got.Name, tc.wantName)
			}
		})
	}
}
