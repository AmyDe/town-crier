package offercodes

import "testing"

func TestRandomGenerator_ProducesCanonicalCodes(t *testing.T) {
	t.Parallel()

	g := NewRandomGenerator()
	seen := make(map[string]struct{})
	for range 200 {
		code, err := g.Generate()
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if !IsValidCanonical(code) {
			t.Fatalf("generated non-canonical code %q", code)
		}
		seen[code] = struct{}{}
	}
	// 60 bits of entropy: 200 draws colliding would be astronomically unlikely,
	// so a near-full distinct set confirms the generator is actually random.
	if len(seen) < 199 {
		t.Errorf("only %d/200 codes were distinct — generator not random enough", len(seen))
	}
}
