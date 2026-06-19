package tc

import "testing"

func TestParseArgs_ParsesCommandName(t *testing.T) {
	t.Parallel()
	got := ParseArgs([]string{"grant-subscription", "--email", "a@b.com"})
	if got.Command != "grant-subscription" {
		t.Fatalf("Command = %q, want grant-subscription", got.Command)
	}
}

func TestParseArgs_ParsesKeyValuePairs(t *testing.T) {
	t.Parallel()
	got := ParseArgs([]string{"grant-subscription", "--email", "a@b.com", "--tier", "Pro"})

	if email, err := got.GetRequired("email"); err != nil || email != "a@b.com" {
		t.Fatalf("email = %q, err = %v, want a@b.com", email, err)
	}
	if tier, err := got.GetRequired("tier"); err != nil || tier != "Pro" {
		t.Fatalf("tier = %q, err = %v, want Pro", tier, err)
	}
}

func TestParseArgs_ReturnsHelp(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
	}{
		{"no args", nil},
		{"-h", []string{"-h"}},
		{"--help", []string{"--help"}},
		{"help", []string{"help"}},
		{"-H uppercase alias", []string{"-H"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ParseArgs(tc.args); got.Command != "help" {
				t.Fatalf("Command = %q, want help", got.Command)
			}
		})
	}
}

func TestParseArgs_ExtractsGlobalArgs(t *testing.T) {
	t.Parallel()
	got := ParseArgs([]string{"grant-subscription", "--url", "http://localhost:8080", "--email", "a@b.com"})

	if url, ok := got.GetOptional("url"); !ok || url != "http://localhost:8080" {
		t.Fatalf("url = %q, ok = %v, want http://localhost:8080", url, ok)
	}
	if email, err := got.GetRequired("email"); err != nil || email != "a@b.com" {
		t.Fatalf("email = %q, err = %v, want a@b.com", email, err)
	}
}

func TestParseArgs_GetRequiredMissingReturnsError(t *testing.T) {
	t.Parallel()
	got := ParseArgs([]string{"grant-subscription"})
	if _, err := got.GetRequired("email"); err == nil {
		t.Fatal("GetRequired(email) = nil error, want error")
	}
}

func TestParseArgs_GetOptionalMissingReturnsFalse(t *testing.T) {
	t.Parallel()
	got := ParseArgs([]string{"grant-subscription"})
	if value, ok := got.GetOptional("url"); ok || value != "" {
		t.Fatalf("GetOptional(url) = (%q, %v), want (\"\", false)", value, ok)
	}
}

func TestParseArgs_TrailingFlagWithoutValueIgnored(t *testing.T) {
	t.Parallel()
	got := ParseArgs([]string{"list-users", "--search", "alice", "--page-size"})

	if search, ok := got.GetOptional("search"); !ok || search != "alice" {
		t.Fatalf("search = %q, ok = %v, want alice", search, ok)
	}
	if _, ok := got.GetOptional("page-size"); ok {
		t.Fatal("page-size should be absent (trailing flag without value ignored)")
	}
}

func TestParseArgs_KeysAreCaseInsensitive(t *testing.T) {
	t.Parallel()
	got := ParseArgs([]string{"grant-subscription", "--Email", "a@b.com"})
	if email, err := got.GetRequired("email"); err != nil || email != "a@b.com" {
		t.Fatalf("email = %q, err = %v, want a@b.com", email, err)
	}
}
