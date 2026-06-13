package authorities

import "testing"

func TestLookup_ByID(t *testing.T) {
	t.Parallel()
	lookup := NewLookup()

	// 471 is City of London, present in the embedded data.
	a, ok := lookup.ByID(471)
	if !ok {
		t.Fatal("expected authority 471 to exist")
	}
	if a.ID != 471 || a.Name == "" || a.AreaType == "" {
		t.Errorf("authority 471 incomplete: %+v", a)
	}

	if _, ok := lookup.ByID(-1); ok {
		t.Error("expected authority -1 to be absent")
	}
}

func TestCompareOrdinalIgnoreCase_Exported(t *testing.T) {
	t.Parallel()
	if CompareOrdinalIgnoreCase("apple", "Banana") >= 0 {
		t.Error("apple should sort before Banana ignoring case")
	}
	if CompareOrdinalIgnoreCase("Zed", "zed") != 0 {
		t.Error("case-only difference should compare equal")
	}
}
