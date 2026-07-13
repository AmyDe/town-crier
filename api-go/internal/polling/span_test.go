package polling

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/planit"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// recordSpans swaps in an in-memory SDK TracerProvider for the duration of
// run, restoring the previous global provider on cleanup, and returns every
// span recorded. Deliberately not t.Parallel(): mutating the global
// TracerProvider is safe only while no sibling test's body is concurrently
// executing (mirrors internal/worker/dispatch_test.go's recordSingleSpan).
func recordSpans(t *testing.T, run func()) []sdktrace.ReadOnlySpan {
	t.Helper()
	prev := otel.GetTracerProvider()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(prev)
	})
	run()
	return rec.Ended()
}

// spanNamed returns the first recorded span with the given name.
func spanNamed(spans []sdktrace.ReadOnlySpan, name string) (sdktrace.ReadOnlySpan, bool) {
	for _, s := range spans {
		if s.Name() == name {
			return s, true
		}
	}
	return nil, false
}

// attrValue returns the raw attribute.Value for key so callers can assert its
// typed accessor (AsInt64/AsBool/AsString).
func attrValue(span sdktrace.ReadOnlySpan, key string) (attribute.Value, bool) {
	for _, kv := range span.Attributes() {
		if string(kv.Key) == key {
			return kv.Value, true
		}
	}
	return attribute.Value{}, false
}

// TestPollAuthority_EmitsSpanWithFullAttributeSet pins the "PlanIt authority
// poll" span's full attribute set (tc-nlvpz / GH#955 PR A) for the common
// natural-end, no-cursor, known-total case.
func TestPollAuthority_EmitsSpanWithFullAttributeSet(t *testing.T) {
	pi := newFakePlanIt()
	// newHandler pins the clock to 2026-06-14T12:00:00Z; a fresh authority (no
	// PollState) defaults existingHWM to one day earlier: 2026-06-13T12:00:00Z.
	// ld is chosen after that default so the natural end genuinely advances HWM.
	ld := time.Date(2026, 6, 13, 15, 0, 0, 0, time.UTC)
	total := 500
	pi.pages[pageKey{authority: 99, index: 0}] = planit.FetchPageResult{
		From: 0, Applications: []applications.PlanningApplication{testApp("a", 99, ld)}, HasMorePages: false, Total: &total,
	}
	apps := newFakeApps()
	state := newFakeStateStore()
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleWatched, defaultHandlerOpts())

	spans := recordSpans(t, func() {
		if _, err := h.Handle(context.Background()); err != nil {
			t.Fatalf("Handle: %v", err)
		}
	})

	span, ok := spanNamed(spans, "PlanIt authority poll")
	if !ok {
		t.Fatalf("expected a %q span among %d recorded spans", "PlanIt authority poll", len(spans))
	}

	wantInt := map[string]int64{
		"polling.authority_id": 99,
		"polling.start_index":  0,
		"polling.next_index":   1, // from(0) + records(1)
		"polling.known_total":  500,
		"polling.returned":     1,
	}
	for key, want := range wantInt {
		v, ok := attrValue(span, key)
		if !ok {
			t.Errorf("missing attribute %q", key)
			continue
		}
		if got := v.AsInt64(); got != want {
			t.Errorf("%s: got %d, want %d", key, got, want)
		}
	}

	wantBool := map[string]bool{
		"polling.cap_hit":      false,
		"polling.rate_limited": false,
		"polling.probe_ran":    false,
		"polling.hwm_advanced": true,
	}
	for key, want := range wantBool {
		v, ok := attrValue(span, key)
		if !ok {
			t.Errorf("missing attribute %q", key)
			continue
		}
		if got := v.AsBool(); got != want {
			t.Errorf("%s: got %v, want %v", key, got, want)
		}
	}

	if v, ok := attrValue(span, "polling.cycle_type"); !ok || v.AsString() != "Watched" {
		t.Errorf("polling.cycle_type: got %v (ok=%v), want Watched", v, ok)
	}
	if v, ok := attrValue(span, "polling.different_start"); !ok || v.AsString() != "2026-06-13" {
		t.Errorf("polling.different_start: got %v (ok=%v), want 2026-06-13", v, ok)
	}
}

// TestPollAuthority_SpanOmitsKnownTotalWhenUnknown covers a response that omits
// total: the span must not carry a misleading polling.known_total=0.
func TestPollAuthority_SpanOmitsKnownTotalWhenUnknown(t *testing.T) {
	pi := newFakePlanIt()
	pi.pages[pageKey{authority: 99, index: 0}] = planit.FetchPageResult{From: 0, HasMorePages: false}
	apps := newFakeApps()
	state := newFakeStateStore()
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, defaultHandlerOpts())

	spans := recordSpans(t, func() {
		if _, err := h.Handle(context.Background()); err != nil {
			t.Fatalf("Handle: %v", err)
		}
	})
	span, ok := spanNamed(spans, "PlanIt authority poll")
	if !ok {
		t.Fatalf("expected a %q span", "PlanIt authority poll")
	}
	if _, ok := attrValue(span, "polling.known_total"); ok {
		t.Error("polling.known_total should be absent when PlanIt omitted total")
	}
}

// TestPollAuthority_SpanTagsCapHitProbeRanAndFrozenHWM covers the cap-hit +
// active-cursor path: polling.cap_hit and polling.probe_ran must both be true,
// and polling.hwm_advanced must be false (a cap hit freezes the HWM rather than
// advancing it).
func TestPollAuthority_SpanTagsCapHitProbeRanAndFrozenHWM(t *testing.T) {
	pi := newFakePlanIt()
	hwm := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)
	pi.pages[pageKey{authority: 99, index: 0, descending: true}] = planit.FetchPageResult{From: 0, HasMorePages: false}
	full := make([]applications.PlanningApplication, 100)
	for i := range full {
		full[i] = testApp("app", 99, hwm)
	}
	pi.pages[pageKey{authority: 99, index: 300}] = planit.FetchPageResult{
		From: 300, Applications: full, HasMorePages: true, Total: platform.Ptr(1000),
	}
	apps := newFakeApps()
	state := newFakeStateStore()
	state.states[99] = PollState{LastPollTime: hwm, HighWaterMark: hwm, Cursor: &PollCursor{DifferentStart: hwm, NextIndex: 400}}
	one := 1
	opts := HandlerOptions{MaxPagesPerAuthorityPerCycle: &one, HandlerBudget: 4 * time.Minute}
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, opts)

	spans := recordSpans(t, func() {
		if _, err := h.Handle(context.Background()); err != nil {
			t.Fatalf("Handle: %v", err)
		}
	})
	span, ok := spanNamed(spans, "PlanIt authority poll")
	if !ok {
		t.Fatalf("expected a %q span", "PlanIt authority poll")
	}
	if v, ok := attrValue(span, "polling.cap_hit"); !ok || !v.AsBool() {
		t.Errorf("polling.cap_hit: got %v (ok=%v), want true", v, ok)
	}
	if v, ok := attrValue(span, "polling.probe_ran"); !ok || !v.AsBool() {
		t.Errorf("polling.probe_ran: got %v (ok=%v), want true", v, ok)
	}
	if v, ok := attrValue(span, "polling.hwm_advanced"); !ok || v.AsBool() {
		t.Errorf("polling.hwm_advanced: got %v (ok=%v), want false (cap-hit freezes HWM)", v, ok)
	}
	if v, ok := attrValue(span, "polling.next_index"); !ok || v.AsInt64() != 400 {
		t.Errorf("polling.next_index: got %v (ok=%v), want 400", v, ok)
	}
}

// TestPollAuthority_EmitsOneSpanPerVisitedAuthority proves the span is emitted
// once per authority, not once per cycle.
func TestPollAuthority_EmitsOneSpanPerVisitedAuthority(t *testing.T) {
	pi := newFakePlanIt()
	ld := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	pi.pages[pageKey{authority: 1, index: 0}] = planit.FetchPageResult{From: 0, Applications: []applications.PlanningApplication{testApp("a", 1, ld)}}
	pi.pages[pageKey{authority: 2, index: 0}] = planit.FetchPageResult{From: 0, Applications: []applications.PlanningApplication{testApp("b", 2, ld)}}
	apps := newFakeApps()
	state := newFakeStateStore()
	h := newHandler(t, pi, apps, state, fakeAuthorities{ids: []int{1, 2}}, CycleSeed, defaultHandlerOpts())

	spans := recordSpans(t, func() {
		if _, err := h.Handle(context.Background()); err != nil {
			t.Fatalf("Handle: %v", err)
		}
	})

	count := 0
	for _, s := range spans {
		if s.Name() == "PlanIt authority poll" {
			count++
		}
	}
	if count != 2 {
		t.Errorf("PlanIt authority poll span count: got %d, want 2 (one per authority)", count)
	}
}
