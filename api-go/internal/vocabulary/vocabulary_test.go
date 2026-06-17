package vocabulary

import "testing"

func TestUKDisplayString(t *testing.T) {
	t.Parallel()

	ptr := func(s string) *string { return &s }

	tests := []struct {
		name  string
		state *string
		want  string
	}{
		{"permitted", ptr("Permitted"), "Approved"},
		{"conditions", ptr("Conditions"), "Approved with conditions"},
		{"rejected", ptr("Rejected"), "Refused"},
		{"appealed", ptr("Appealed"), "Refusal appealed"},
		{"case-insensitive", ptr("permitted"), "Approved"},
		{"trims whitespace", ptr("  Rejected  "), "Refused"},
		{"nil", nil, ""},
		{"unknown", ptr("Withdrawn"), ""},
		{"empty", ptr(""), ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := UKDisplayString(tc.state); got != tc.want {
				t.Fatalf("UKDisplayString(%v) = %q, want %q", tc.state, got, tc.want)
			}
		})
	}
}
