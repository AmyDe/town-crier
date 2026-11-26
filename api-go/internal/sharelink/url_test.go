package sharelink

import "testing"

func TestApplicationURL(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, slug, ref, want string
	}{
		{"keeps slashes", "leeds", "26/01234/FUL", "https://share.towncrierapp.uk/a/leeds/26/01234/FUL"},
		{"escapes each part", "leeds", "A B/c?d#e&é", "https://share.towncrierapp.uk/a/leeds/A%20B/c%3Fd%23e%26%C3%A9"},
		{"leaves encodeURIComponent marks", "leeds", "-_.!~*'()", "https://share.towncrierapp.uk/a/leeds/-_.!~*'()"},
		{"no slug", "", "26/1", ""},
		{"no reference", "leeds", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ApplicationURL(tc.slug, tc.ref); got != tc.want {
				t.Errorf("ApplicationURL(%q, %q) = %q, want %q", tc.slug, tc.ref, got, tc.want)
			}
		})
	}
}
