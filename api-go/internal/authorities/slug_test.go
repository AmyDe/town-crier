package authorities

import (
	"encoding/json"
	"os"
	"testing"
)

// slugFixturePath points at the shared JS/Go slug ground-truth vectors. It is
// relative to this package's directory (api-go/internal/authorities); go test
// runs each package with its own directory as the working directory, so three
// levels up reaches the repo root where web/scripts/lib/slug-fixtures.json lives.
const slugFixturePath = "../../../web/scripts/lib/slug-fixtures.json"

// TestSlugify_MatchesSharedFixture guards the Go port against drift from the
// canonical JS algorithm in web/scripts/lib/slug.mjs: both read the SAME fixture.
func TestSlugify_MatchesSharedFixture(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(slugFixturePath)
	if err != nil {
		t.Fatalf("read shared slug fixture %s: %v (the byte-equal parity fixture must be present in this worktree/PR)", slugFixturePath, err)
	}

	var vectors []struct {
		Input    string `json:"input"`
		Expected string `json:"expected"`
	}
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatalf("unmarshal shared slug fixture %s: %v", slugFixturePath, err)
	}
	if len(vectors) == 0 {
		t.Fatalf("shared slug fixture %s is empty; expected at least one ground-truth vector", slugFixturePath)
	}

	for _, v := range vectors {
		t.Run(v.Input, func(t *testing.T) {
			t.Parallel()
			if got := Slugify(v.Input); got != v.Expected {
				t.Errorf("Slugify(%q) = %q, want %q (drift from web/scripts/lib/slug.mjs)", v.Input, got, v.Expected)
			}
		})
	}
}

// TestSlugify_Cases documents intent directly, redundant with the fixture but
// pinning the specific behaviours the port must honour.
func TestSlugify_Cases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"lowercases and hyphenates words", "Basingstoke and Deane", "basingstoke-and-deane"},
		{"ampersand expands to and", "Hammersmith & Fulham", "hammersmith-and-fulham"},
		{"ascii apostrophe stripped not hyphenated", "King's Lynn", "kings-lynn"},
		{"unicode right single quote stripped", "King’s Lynn", "kings-lynn"},
		{"comma and whitespace collapse", "Bristol, City of", "bristol-city-of"},
		{"leading and trailing separators trimmed", "-- Foo --", "foo"},
		{"internal whitespace runs collapse", "  Test   Name  ", "test-name"},
		{"digits preserved", "Test 123", "test-123"},
		{"existing hyphens preserved", "Stratford-on-Avon", "stratford-on-avon"},
		{"empty string stays empty", "", ""},
		{"non-ascii letters act as separators", "Café Royal", "caf-royal"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := Slugify(tc.in); got != tc.want {
				t.Errorf("Slugify(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestLookup_SlugResolver exercises the slug<->area-id resolution exposed on the
// cross-package Lookup handle.
func TestLookup_SlugResolver(t *testing.T) {
	t.Parallel()
	lookup := NewLookup()

	// Aberdeen (id 384) and Adur (id 245) are stable entries in the embedded
	// authorities data (see store_test.go), so their slugs are known-good.
	known := []struct {
		id   int
		slug string
	}{
		{384, "aberdeen"},
		{245, "adur"},
	}
	for _, k := range known {
		t.Run(k.slug, func(t *testing.T) {
			t.Parallel()

			gotID, ok := lookup.SlugToAreaID(k.slug)
			if !ok || gotID != k.id {
				t.Errorf("SlugToAreaID(%q) = (%d, %v), want (%d, true)", k.slug, gotID, ok, k.id)
			}

			gotSlug, ok := lookup.SlugForAreaID(k.id)
			if !ok || gotSlug != k.slug {
				t.Errorf("SlugForAreaID(%d) = (%q, %v), want (%q, true)", k.id, gotSlug, ok, k.slug)
			}
		})
	}

	t.Run("unknown slug", func(t *testing.T) {
		t.Parallel()
		if id, ok := lookup.SlugToAreaID("no-such-authority-slug"); ok || id != 0 {
			t.Errorf("SlugToAreaID(unknown) = (%d, %v), want (0, false)", id, ok)
		}
	})

	t.Run("unknown id", func(t *testing.T) {
		t.Parallel()
		if slug, ok := lookup.SlugForAreaID(-1); ok || slug != "" {
			t.Errorf("SlugForAreaID(-1) = (%q, %v), want (\"\", false)", slug, ok)
		}
	})
}
