package offercodes

import (
	"errors"
	"testing"
)

func TestNormalize_StripsSeparatorsAndUpperCases(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"ABCD-EFGH-JKMN":  "ABCDEFGHJKMN",
		"abcd efgh jkmn":  "ABCDEFGHJKMN",
		"abcdefghjkmn":    "ABCDEFGHJKMN",
		"  ABCDEFGHJKMN ": "ABCDEFGHJKMN",
	}
	for in, want := range tests {
		got, err := Normalize(in)
		if err != nil {
			t.Errorf("Normalize(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalize_RejectsWithDotNetMessages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"blank", "   ", "Offer code is required."},
		{"empty", "", "Offer code is required."},
		{"invalid char", "ABCD-EFGH-JKM!", "Offer code contains invalid character '!'."},
		{"ambiguous letter I", "ABCDEFGHJKMI", "Offer code contains invalid character 'I'."},
		{"too short", "ABCDEFGH", "Offer code must be 12 characters (got 8)."},
		{"too long", "ABCDEFGHJKMNPQ", "Offer code must be 12 characters (got 14)."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := Normalize(tc.in)
			var fe *InvalidFormatError
			if !errors.As(err, &fe) {
				t.Fatalf("Normalize(%q) error = %v, want *InvalidFormatError", tc.in, err)
			}
			if fe.Message != tc.want {
				t.Errorf("message = %q, want %q", fe.Message, tc.want)
			}
		})
	}
}

func TestFormat_RendersDisplayForm(t *testing.T) {
	t.Parallel()

	if got := Format("ABCDEFGHJKMN"); got != "ABCD-EFGH-JKMN" {
		t.Errorf("Format = %q, want ABCD-EFGH-JKMN", got)
	}
}

func TestIsValidCanonical(t *testing.T) {
	t.Parallel()

	tests := map[string]bool{
		"ABCDEFGHJKMN":   true,
		"000000000000":   true,
		"ABCDEFGHJKM":    false, // 11 chars
		"ABCDEFGHJKMNP":  false, // 13 chars
		"ABCD-EFGH-JKMN": false, // separators not canonical
		"ABCDEFGHJKMI":   false, // I is not in the alphabet
	}
	for in, want := range tests {
		if got := IsValidCanonical(in); got != want {
			t.Errorf("IsValidCanonical(%q) = %v, want %v", in, got, want)
		}
	}
}
