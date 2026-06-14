package polling

import (
	"testing"
	"time"
)

func TestRandomJitter_NextOffsetWithinBound(t *testing.T) {
	t.Parallel()
	j := NewRandomJitter()
	bound := 10 * time.Second
	for range 1000 {
		off := j.NextOffset(bound)
		if off < -bound || off > bound {
			t.Fatalf("offset %v out of [-%v, +%v]", off, bound, bound)
		}
	}
}

func TestRandomJitter_NonPositiveBoundIsZero(t *testing.T) {
	t.Parallel()
	j := NewRandomJitter()
	if got := j.NextOffset(0); got != 0 {
		t.Errorf("zero bound: got %v, want 0", got)
	}
	if got := j.NextOffset(-5 * time.Second); got != 0 {
		t.Errorf("negative bound: got %v, want 0", got)
	}
}
