package geocoding

import "testing"

func TestNormalisePostcode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		raw    string
		want   string
		wantOK bool
	}{
		{"valid with space", "sw1a 1aa", "SW1A 1AA", true},
		{"valid no space", "ec2v5ae", "EC2V5AE", true},
		{"valid lowercase trimmed", "  n1 9gu  ", "N1 9GU", true},
		{"blank", "   ", "", false},
		{"empty", "", "", false},
		{"malformed letters", "NOTAPOSTCODE", "", false},
		{"malformed digits", "12345", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := normalisePostcode(tc.raw)
			if ok != tc.wantOK {
				t.Fatalf("ok: got %v, want %v", ok, tc.wantOK)
			}
			if got != tc.want {
				t.Errorf("normalised: got %q, want %q", got, tc.want)
			}
		})
	}
}
