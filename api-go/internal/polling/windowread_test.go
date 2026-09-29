package polling

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

type fakeWindowFetcher struct {
	pages   map[int]planit.FetchPageResult
	errAt   map[int]error
	queries []planit.WindowQuery
}

func (f *fakeWindowFetcher) FetchPage(_ context.Context, q planit.WindowQuery) (planit.FetchPageResult, error) {
	f.queries = append(f.queries, q)
	if err := f.errAt[q.Index]; err != nil {
		return planit.FetchPageResult{}, err
	}
	p, ok := f.pages[q.Index]
	if !ok {
		return planit.FetchPageResult{}, fmt.Errorf("fake: no page at index %d", q.Index)
	}
	return p, nil
}

type fakeAppIngester struct {
	got []applications.PlanningApplication
	err error
}

func (f *fakeAppIngester) Ingest(_ context.Context, app applications.PlanningApplication) error {
	if f.err != nil {
		return f.err
	}
	f.got = append(f.got, app)
	return nil
}

type fakeWindowWriter struct {
	probes []time.Time
	fulls  []fullRead
}

type fullRead struct {
	at    time.Time
	total int
}

func (f *fakeWindowWriter) MarkProbeComplete(_ context.Context, _ WindowRef, at time.Time) error {
	f.probes = append(f.probes, at)
	return nil
}

func (f *fakeWindowWriter) MarkFullRead(_ context.Context, _ WindowRef, at time.Time, total int) error {
	f.fulls = append(f.fulls, fullRead{at, total})
	return nil
}

func recs(area, from, n int) []applications.PlanningApplication {
	out := make([]applications.PlanningApplication, n)
	for i := range out {
		out[i] = applications.PlanningApplication{UID: fmt.Sprintf("u%d", from+i), AreaID: area}
	}
	return out
}

// page builds a page starting at from with n records out of total.
func page(from, n, total int) planit.FetchPageResult {
	t := total
	return planit.FetchPageResult{From: from, Applications: recs(1, from, n), Total: &t, HasMorePages: from+n < total}
}

type readerRig struct {
	fetch  *fakeWindowFetcher
	ingest *fakeAppIngester
	writer *fakeWindowWriter
	reader *WindowReader
	logs   *bytes.Buffer
	now    time.Time
}

func newReaderRig(t *testing.T, pages map[int]planit.FetchPageResult) *readerRig {
	t.Helper()
	r := &readerRig{
		fetch:  &fakeWindowFetcher{pages: pages, errAt: map[int]error{}},
		ingest: &fakeAppIngester{},
		writer: &fakeWindowWriter{},
		logs:   &bytes.Buffer{},
		now:    londonAt(6, 10, 21, 0),
	}
	log := slog.New(slog.NewTextHandler(r.logs, nil))
	r.reader = NewWindowReader(r.fetch, r.ingest, r.writer, WindowReadConfig{FullReadMaxAge: 7 * 24 * time.Hour}, func() time.Time { return r.now }, log)
	return r
}

var testDay = utcDay(2026, 6, 9)

func testWindow() PlannedWork { return *windowWork(planit.AxisStart, testDay) }

func TestWindowReader_ProbeWhenTotalUnchangedAndRecentFullRead(t *testing.T) {
	t.Parallel()
	r := newReaderRig(t, map[int]planit.FetchPageResult{0: page(0, 300, 900)})
	state := WindowState{LastTotal: intPtr(900), LastFullReadAt: tp(r.now.AddDate(0, 0, -3))}

	res := r.reader.ReadWindow(context.Background(), testWindow(), state)

	if !res.Complete || !res.Probe || res.Pages != 1 || res.Stop != "" {
		t.Fatalf("res = %+v", res)
	}
	if len(r.fetch.queries) != 1 {
		t.Fatalf("queries = %d, want 1", len(r.fetch.queries))
	}
	if len(r.ingest.got) != 300 {
		t.Fatalf("page 0 must still be ingested, got %d", len(r.ingest.got))
	}
	if len(r.writer.probes) != 1 || !r.writer.probes[0].Equal(r.now) || len(r.writer.fulls) != 0 {
		t.Fatalf("writer = %+v", r.writer)
	}
}

func TestWindowReader_FullReadWhenTotalChangedOrStale(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		state func(now time.Time) WindowState
	}{
		{"never read", func(time.Time) WindowState { return WindowState{} }},
		{"total changed", func(now time.Time) WindowState {
			return WindowState{LastTotal: intPtr(599), LastFullReadAt: tp(now.AddDate(0, 0, -1))}
		}},
		{"full read older than max age", func(now time.Time) WindowState {
			return WindowState{LastTotal: intPtr(600), LastFullReadAt: tp(now.AddDate(0, 0, -8))}
		}},
		{"full read never recorded", func(time.Time) WindowState { return WindowState{LastTotal: intPtr(600)} }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newReaderRig(t, map[int]planit.FetchPageResult{0: page(0, 300, 600), 300: page(300, 300, 600)})

			res := r.reader.ReadWindow(context.Background(), testWindow(), tc.state(r.now))

			if !res.Complete || res.Probe || res.Pages != 2 || res.Stop != "" {
				t.Fatalf("res = %+v", res)
			}
			if len(r.ingest.got) != 600 {
				t.Fatalf("ingested %d, want 600", len(r.ingest.got))
			}
			if len(r.writer.fulls) != 1 || r.writer.fulls[0].total != 600 || len(r.writer.probes) != 0 {
				t.Fatalf("writer = %+v", r.writer)
			}
			q := r.fetch.queries[1]
			if q.Index != 300 || q.Work != planit.WorkWindowStart || !q.From.Equal(testDay) || !q.To.Equal(testDay) {
				t.Fatalf("second query = %+v", q)
			}
		})
	}
}

func TestWindowReader_FullReadMaxAgeZeroAlwaysReadsEverything(t *testing.T) {
	t.Parallel()
	r := newReaderRig(t, map[int]planit.FetchPageResult{0: page(0, 100, 100)})
	r.reader.cfg.FullReadMaxAge = 0
	state := WindowState{LastTotal: intPtr(100), LastFullReadAt: tp(r.now)}

	res := r.reader.ReadWindow(context.Background(), testWindow(), state)

	if res.Probe || len(r.writer.fulls) != 1 {
		t.Fatalf("res = %+v writer = %+v", res, r.writer)
	}
}

func TestWindowReader_ShortReadLeavesWindowDue(t *testing.T) {
	t.Parallel()
	r := newReaderRig(t, map[int]planit.FetchPageResult{0: page(0, 300, 700), 300: page(300, 300, 700), 600: endPage(600, 50, 700)})

	res := r.reader.ReadWindow(context.Background(), testWindow(), WindowState{})

	if res.Complete || !res.Short || res.Stop != "" {
		t.Fatalf("res = %+v", res)
	}
	if len(r.writer.fulls) != 0 || len(r.writer.probes) != 0 {
		t.Fatalf("a short read must not mark anything, got %+v", r.writer)
	}
	if !strings.Contains(r.logs.String(), "poll.window_short") {
		t.Fatalf("logs = %s", r.logs.String())
	}
}

func TestWindowReader_DuplicatesDoNotCountTowardsCompleteness(t *testing.T) {
	t.Parallel()
	dup := page(0, 300, 400)
	second := page(300, 100, 400)
	second.Applications = recs(1, 0, 100) // repeats uids u0..u99
	r := newReaderRig(t, map[int]planit.FetchPageResult{0: dup, 300: second})

	res := r.reader.ReadWindow(context.Background(), testWindow(), WindowState{})

	if res.Complete || !res.Short {
		t.Fatalf("res = %+v", res)
	}
}

func TestWindowReader_SameUIDDifferentAreaCountsSeparately(t *testing.T) {
	t.Parallel()
	total := 2
	p := planit.FetchPageResult{Total: &total, Applications: []applications.PlanningApplication{
		{UID: "x", AreaID: 1}, {UID: "x", AreaID: 2},
	}}
	r := newReaderRig(t, map[int]planit.FetchPageResult{0: p})

	res := r.reader.ReadWindow(context.Background(), testWindow(), WindowState{})

	if !res.Complete {
		t.Fatalf("res = %+v", res)
	}
}

func TestWindowReader_TwoShortReadsInOneBudgetDayAreReported(t *testing.T) {
	t.Parallel()
	r := newReaderRig(t, map[int]planit.FetchPageResult{0: page(0, 10, 20), 10: endPage(10, 5, 20)})
	ctx := context.Background()

	r.reader.ReadWindow(ctx, testWindow(), WindowState{})
	if got := r.reader.ShortWindows(r.now); len(got) != 0 {
		t.Fatalf("one short read is not enough: %v", got)
	}
	r.now = r.now.Add(time.Hour)
	r.reader.ReadWindow(ctx, testWindow(), WindowState{})

	got := r.reader.ShortWindows(r.now)
	if len(got) != 1 || got[0] != (WindowRef{planit.AxisStart, testDay}) {
		t.Fatalf("ShortWindows = %v", got)
	}

	next := londonAt(6, 11, 18, 5)
	if got := r.reader.ShortWindows(next); len(got) != 0 {
		t.Fatalf("a new budget day starts clean: %v", got)
	}
}

func TestWindowReader_StopReasons(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want StopReason
	}{
		{"backed off", fmt.Errorf("x: %w", ErrBackedOff), StopBackoff},
		{"budget", fmt.Errorf("x: %w", ErrBudgetExhausted), StopDailyCap},
		{"429", &planit.RateLimitError{}, StopRateLimited},
		{"403", &planit.ForbiddenError{}, StopForbidden},
		{"timeout", fmt.Errorf("x: %w", planit.ErrTimeout), StopTimeout},
		{"other", errors.New("boom"), StopError},
	}
	for _, tc := range tests {
		t.Run(tc.name+" on page 0", func(t *testing.T) {
			t.Parallel()
			r := newReaderRig(t, nil)
			r.fetch.errAt[0] = tc.err

			res := r.reader.ReadWindow(context.Background(), testWindow(), WindowState{})

			if res.Stop != tc.want || res.Complete || res.Err == nil {
				t.Fatalf("res = %+v", res)
			}
		})
		t.Run(tc.name+" mid window leaves state untouched", func(t *testing.T) {
			t.Parallel()
			r := newReaderRig(t, map[int]planit.FetchPageResult{0: page(0, 300, 900)})
			r.fetch.errAt[300] = tc.err

			res := r.reader.ReadWindow(context.Background(), testWindow(), WindowState{})

			if res.Stop != tc.want || res.Complete {
				t.Fatalf("res = %+v", res)
			}
			if len(r.writer.fulls) != 0 || len(r.writer.probes) != 0 {
				t.Fatalf("writer = %+v", r.writer)
			}
		})
	}
}

func TestWindowReader_IngestFailureStopsWithError(t *testing.T) {
	t.Parallel()
	r := newReaderRig(t, map[int]planit.FetchPageResult{0: page(0, 3, 3)})
	r.ingest.err = errors.New("db down")

	res := r.reader.ReadWindow(context.Background(), testWindow(), WindowState{})

	if res.Stop != StopError || res.Complete || len(r.writer.fulls) != 0 {
		t.Fatalf("res = %+v", res)
	}
}

func TestWindowReader_ReadDeltaReadsToEnd(t *testing.T) {
	t.Parallel()
	r := newReaderRig(t, map[int]planit.FetchPageResult{0: page(0, 300, 650), 300: page(300, 300, 650), 600: page(600, 50, 650)})
	w := NextWindowWork(plannerTestConfig(), WindowPlanState{}, londonAt(6, 10, 9, 5), 0)

	res := r.reader.ReadDelta(context.Background(), *w)

	if res.Pages != 3 || res.Capped || res.Stop != "" || len(r.ingest.got) != 650 {
		t.Fatalf("res = %+v ingested %d", res, len(r.ingest.got))
	}
	q := r.fetch.queries[0]
	if q.Work != planit.WorkDeltaStart || q.DifferentStart == nil || !q.From.Equal(w.From) {
		t.Fatalf("query = %+v", q)
	}
	if len(r.writer.probes)+len(r.writer.fulls) != 0 {
		t.Fatal("deltas keep no window state")
	}
}

func TestWindowReader_ReadDeltaCapsPages(t *testing.T) {
	t.Parallel()
	r := newReaderRig(t, map[int]planit.FetchPageResult{0: page(0, 300, 900), 300: page(300, 300, 900), 600: page(600, 300, 900)})
	w := PlannedWork{Work: planit.WorkDeltaDecided, Axis: planit.AxisDecided, From: testDay, DifferentStart: tp(testDay), MaxPages: 2}

	res := r.reader.ReadDelta(context.Background(), w)

	if res.Pages != 2 || !res.Capped || res.Stop != "" || len(r.fetch.queries) != 2 {
		t.Fatalf("res = %+v", res)
	}
}

func TestWindowReader_ReadDeltaStopReason(t *testing.T) {
	t.Parallel()
	r := newReaderRig(t, nil)
	r.fetch.errAt[0] = &planit.RateLimitError{}
	w := PlannedWork{Work: planit.WorkDeltaStart, MaxPages: 5, DifferentStart: tp(testDay)}

	res := r.reader.ReadDelta(context.Background(), w)

	if res.Stop != StopRateLimited {
		t.Fatalf("res = %+v", res)
	}
}

// endPage is a final page that ends the read although fewer records than total were returned.
func endPage(from, n, total int) planit.FetchPageResult {
	p := page(from, n, total)
	p.HasMorePages = false
	return p
}

type recordingHooks struct{ calls []string }

func (h *recordingHooks) PageFetched(context.Context, planit.WindowQuery, planit.FetchPageResult) {
	h.calls = append(h.calls, "fetched")
}

func (h *recordingHooks) PageIngested(context.Context, planit.WindowQuery, planit.FetchPageResult) error {
	h.calls = append(h.calls, "ingested")
	return nil
}

func (h *recordingHooks) WindowShort(context.Context, WindowRef, int, int) {
	h.calls = append(h.calls, "short")
}

func (h *recordingHooks) FullReadComplete(_ context.Context, _ WindowRef, started, done time.Time, read map[AppKey]struct{}) error {
	h.calls = append(h.calls, fmt.Sprintf("full:%d:%s:%s", len(read), started.Format("15:04"), done.Format("15:04")))
	return nil
}

func TestWindowReader_HooksSeeEveryPageAndTheFullRead(t *testing.T) {
	t.Parallel()
	r := newReaderRig(t, map[int]planit.FetchPageResult{0: page(0, 300, 350), 300: page(300, 50, 350)})
	h := &recordingHooks{}
	r.reader.WithHooks(h)

	res := r.reader.ReadWindow(context.Background(), testWindow(), WindowState{})

	if !res.Complete {
		t.Fatalf("res = %+v", res)
	}
	want := []string{"fetched", "ingested", "fetched", "ingested", "full:350:21:00:21:00"}
	if fmt.Sprint(h.calls) != fmt.Sprint(want) {
		t.Fatalf("calls = %v, want %v", h.calls, want)
	}
}

func TestWindowReader_HooksSeeAShortRead(t *testing.T) {
	t.Parallel()
	last := page(300, 50, 400)
	last.HasMorePages = false
	r := newReaderRig(t, map[int]planit.FetchPageResult{0: page(0, 300, 400), 300: last})
	h := &recordingHooks{}
	r.reader.WithHooks(h)

	res := r.reader.ReadWindow(context.Background(), testWindow(), WindowState{})

	if !res.Short || h.calls[len(h.calls)-1] != "short" {
		t.Fatalf("res = %+v calls = %v", res, h.calls)
	}
}

func TestWindowReader_ProbeAndDeltaSkipFullReadHook(t *testing.T) {
	t.Parallel()
	r := newReaderRig(t, map[int]planit.FetchPageResult{0: page(0, 300, 900)})
	h := &recordingHooks{}
	r.reader.WithHooks(h)
	state := WindowState{LastTotal: intPtr(900), LastFullReadAt: tp(r.now.AddDate(0, 0, -3))}

	r.reader.ReadWindow(context.Background(), testWindow(), state)

	for _, c := range h.calls {
		if strings.HasPrefix(c, "full") {
			t.Fatalf("probe must not fire the full-read hook: %v", h.calls)
		}
	}
}
