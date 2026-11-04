package authorities

import "testing"

func TestCompareOrdinalIgnoreCase_Exported(t *testing.T) {
	t.Parallel()
	if CompareOrdinalIgnoreCase("apple", "Banana") >= 0 {
		t.Error("apple should sort before Banana ignoring case")
	}
	if CompareOrdinalIgnoreCase("Zed", "zed") != 0 {
		t.Error("case-only difference should compare equal")
	}
}
