package polling

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/appevents"
	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

type fakeDeltaSeen struct {
	upserts []DeltaSeen
	take    []AppKey
	taken   []WindowRef
	before  []time.Time
}

func (f *fakeDeltaSeen) Upsert(_ context.Context, rows []DeltaSeen) error {
	f.upserts = append(f.upserts, rows...)
	return nil
}

func (f *fakeDeltaSeen) Take(_ context.Context, ref WindowRef, before time.Time) ([]AppKey, error) {
	f.taken = append(f.taken, ref)
	f.before = append(f.before, before)
	return f.take, nil
}

type fakeMembers struct {
	replaced map[WindowRef][]AppKey
	readAt   map[WindowRef]time.Time
}

func (f *fakeMembers) Replace(_ context.Context, ref WindowRef, readAt time.Time, keys []AppKey) error {
	if f.replaced == nil {
		f.replaced, f.readAt = map[WindowRef][]AppKey{}, map[WindowRef]time.Time{}
	}
	f.replaced[ref], f.readAt[ref] = keys, readAt
	return nil
}

type fakeEvents struct{ got []PollEvent }

func (f *fakeEvents) Record(_ context.Context, e PollEvent) error {
	f.got = append(f.got, e)
	return nil
}

type fakePageDispatcher struct {
	calls int
	res   appevents.Result
}

func (f *fakePageDispatcher) Dispatch(context.Context) (appevents.Result, error) {
	f.calls++
	return f.res, nil
}

type observerRig struct {
	obs   *RunObserver
	delta *fakeDeltaSeen
	mem   *fakeMembers
	evs   *fakeEvents
	disp  *fakePageDispatcher
	logs  *bytes.Buffer
	now   time.Time
}

func newObserverRig(t *testing.T, withMembers bool) *observerRig {
	t.Helper()
	r := &observerRig{delta: &fakeDeltaSeen{}, mem: &fakeMembers{}, evs: &fakeEvents{}, disp: &fakePageDispatcher{}, logs: &bytes.Buffer{}, now: londonAt(6, 10, 21, 0)}
	var members memberStore
	if withMembers {
		members = r.mem
	}
	r.obs = NewRunObserver(r.delta, members, r.evs, r.disp, func() time.Time { return r.now }, slog.New(slog.NewTextHandler(r.logs, nil)))
	return r
}

func appOn(uid string, start, decided *time.Time) applications.PlanningApplication {
	return applications.PlanningApplication{UID: uid, AreaID: 1, StartDate: start, DecidedDate: decided}
}

func dayPtr(m time.Month, d int) *time.Time { t := utcDay(2026, m, d); return &t }

func pageOf(apps ...applications.PlanningApplication) planit.FetchPageResult {
	return planit.FetchPageResult{Applications: apps}
}

func windowQuery(day time.Time) planit.WindowQuery {
	return planit.WindowQuery{Work: planit.WorkWindowStart, Axis: planit.AxisStart, From: day, To: day}
}

func TestRunObserver_WindowIntegrity(t *testing.T) {
	t.Parallel()
	day := utcDay(2026, 6, 9)
	tests := []struct {
		name string
		page planit.FetchPageResult
		want int
	}{
		{"all inside", pageOf(appOn("a", &day, nil), appOn("b", &day, nil)), 0},
		{"one outside", pageOf(appOn("a", &day, nil), appOn("b", dayPtr(6, 8), nil)), 1},
		{"null axis date", pageOf(appOn("a", nil, dayPtr(6, 9))), 1},
		{"two outside", pageOf(appOn("a", dayPtr(6, 10), nil), appOn("b", dayPtr(6, 1), nil)), 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newObserverRig(t, false)
			r.obs.PageFetched(context.Background(), windowQuery(day), tt.page)
			if got := r.obs.Counts().Violations; got != tt.want {
				t.Fatalf("violations = %d, want %d", got, tt.want)
			}
			if tt.want == 0 {
				if len(r.evs.got) != 0 || r.logs.Len() != 0 {
					t.Fatalf("clean page must be silent: %+v %q", r.evs.got, r.logs.String())
				}
				return
			}
			if len(r.evs.got) != 1 || r.evs.got[0].Kind != EventWindowViolation || r.evs.got[0].Detail != tt.want {
				t.Fatalf("events = %+v", r.evs.got)
			}
			out := r.logs.String()
			if !strings.Contains(out, "level=ERROR") || !strings.Contains(out, "poll.window_violation") || !strings.Contains(out, "2026-06-09") {
				t.Fatalf("log = %q", out)
			}
		})
	}
}

func TestRunObserver_DeltaMaskIntegrity(t *testing.T) {
	t.Parallel()
	mask := utcDay(2026, 5, 27)
	diff := utcDay(2026, 6, 9)
	q := planit.WindowQuery{Work: planit.WorkDeltaDecided, Axis: planit.AxisDecided, From: mask, DifferentStart: &diff}
	r := newObserverRig(t, false)

	r.obs.PageFetched(context.Background(), q, pageOf(
		appOn("ok", nil, dayPtr(6, 1)),
		appOn("old", nil, dayPtr(5, 26)),
	))

	if got := r.obs.Counts().Violations; got != 1 {
		t.Fatalf("violations = %d, want 1", got)
	}
}

func TestRunObserver_DeltaPageWritesDeltaSeenForBandRecordsOnly(t *testing.T) {
	t.Parallel()
	r := newObserverRig(t, false)
	diff := utcDay(2026, 6, 9)
	q := planit.WindowQuery{Work: planit.WorkDeltaStart, Axis: planit.AxisStart, From: utcDay(2026, 5, 27), DifferentStart: &diff}

	err := r.obs.PageIngested(context.Background(), q, pageOf(
		appOn("today", dayPtr(6, 10), nil),
		appOn("edge", dayPtr(5, 27), nil),
		appOn("outside", dayPtr(5, 26), nil),
		appOn("nodate", nil, nil),
	))
	if err != nil {
		t.Fatal(err)
	}

	if len(r.delta.upserts) != 2 {
		t.Fatalf("upserts = %+v", r.delta.upserts)
	}
	first := r.delta.upserts[0]
	if first.Axis != planit.AxisStart || first.UID != "today" || first.AreaID != 1 || !first.Day.Equal(utcDay(2026, 6, 10)) || !first.SeenAt.Equal(r.now) {
		t.Fatalf("row = %+v", first)
	}
}

func TestRunObserver_WindowPageWritesNoDeltaSeenAndDispatches(t *testing.T) {
	t.Parallel()
	r := newObserverRig(t, false)
	day := utcDay(2026, 6, 9)

	if err := r.obs.PageIngested(context.Background(), windowQuery(day), pageOf(appOn("a", &day, nil))); err != nil {
		t.Fatal(err)
	}

	if len(r.delta.upserts) != 0 {
		t.Fatalf("window pages must not write delta_seen: %+v", r.delta.upserts)
	}
	if r.disp.calls != 1 {
		t.Fatalf("dispatch calls = %d, want 1", r.disp.calls)
	}
}

func TestRunObserver_SurgeIsRecordedOnce(t *testing.T) {
	t.Parallel()
	r := newObserverRig(t, false)
	r.disp.res = appevents.Result{Surge: true}
	day := utcDay(2026, 6, 9)
	q := windowQuery(day)

	_ = r.obs.PageIngested(context.Background(), q, pageOf())
	_ = r.obs.PageIngested(context.Background(), q, pageOf())

	if !r.obs.Counts().Surge {
		t.Fatal("surge not counted")
	}
	surges := 0
	for _, e := range r.evs.got {
		if e.Kind == EventSurge {
			surges++
		}
	}
	if surges != 2 {
		t.Fatalf("surge events = %d, want one per tripped dispatch", surges)
	}
}

func TestRunObserver_FullReadCountsDeltaSeenRecordsTheWindowMissed(t *testing.T) {
	t.Parallel()
	r := newObserverRig(t, false)
	ref := WindowRef{Axis: planit.AxisStart, Day: utcDay(2026, 6, 9)}
	started := londonAt(6, 10, 20, 0)
	r.delta.take = []AppKey{{"a", 1}, {"b", 1}}

	err := r.obs.FullReadComplete(context.Background(), ref, started, r.now, map[AppKey]struct{}{{"a", 1}: {}})
	if err != nil {
		t.Fatal(err)
	}

	if got := r.obs.Counts().Missed; got != 1 {
		t.Fatalf("missed = %d, want 1", got)
	}
	if len(r.delta.taken) != 1 || r.delta.taken[0] != ref || !r.delta.before[0].Equal(started) {
		t.Fatalf("take = %+v %+v", r.delta.taken, r.delta.before)
	}
	if len(r.evs.got) != 1 || r.evs.got[0].Kind != EventWindowMissed || r.evs.got[0].Detail != 1 {
		t.Fatalf("events = %+v", r.evs.got)
	}
	if !strings.Contains(r.logs.String(), "level=ERROR") {
		t.Fatalf("missed must log at error level: %q", r.logs.String())
	}
}

func TestRunObserver_FullReadWithNothingMissedIsSilent(t *testing.T) {
	t.Parallel()
	r := newObserverRig(t, false)
	r.delta.take = []AppKey{{"a", 1}}

	_ = r.obs.FullReadComplete(context.Background(), WindowRef{Day: utcDay(2026, 6, 9)}, r.now, r.now, map[AppKey]struct{}{{"a", 1}: {}})

	if r.obs.Counts().Missed != 0 || len(r.evs.got) != 0 {
		t.Fatalf("missed=%d events=%+v", r.obs.Counts().Missed, r.evs.got)
	}
}

func TestRunObserver_MembersReplacedOnlyWhenConfigured(t *testing.T) {
	t.Parallel()
	ref := WindowRef{Axis: planit.AxisDecided, Day: utcDay(2026, 6, 9)}
	read := map[AppKey]struct{}{{"a", 1}: {}, {"b", 2}: {}}

	dev := newObserverRig(t, true)
	if err := dev.obs.FullReadComplete(context.Background(), ref, dev.now, dev.now, read); err != nil {
		t.Fatal(err)
	}
	if len(dev.mem.replaced[ref]) != 2 || !dev.mem.readAt[ref].Equal(dev.now) {
		t.Fatalf("members = %+v", dev.mem)
	}

	prod := newObserverRig(t, false)
	if err := prod.obs.FullReadComplete(context.Background(), ref, prod.now, prod.now, read); err != nil {
		t.Fatal(err)
	}
	if len(prod.mem.replaced) != 0 {
		t.Fatalf("prod must not write members: %+v", prod.mem)
	}
}

func TestRunObserver_ShortIsRecorded(t *testing.T) {
	t.Parallel()
	r := newObserverRig(t, false)
	ref := WindowRef{Axis: planit.AxisDecided, Day: utcDay(2026, 6, 9)}

	r.obs.WindowShort(context.Background(), ref, 90, 100)

	if len(r.evs.got) != 1 {
		t.Fatalf("events = %+v", r.evs.got)
	}
	e := r.evs.got[0]
	if e.Kind != EventWindowShort || e.Axis != planit.AxisDecided || e.Day == nil || !e.Day.Equal(ref.Day) || !e.At.Equal(r.now) {
		t.Fatalf("event = %+v", e)
	}
}

func TestRunObserver_DispatchFailureDoesNotFailThePage(t *testing.T) {
	t.Parallel()
	r := newObserverRig(t, false)
	r.obs = NewRunObserver(r.delta, nil, r.evs, failingDispatcher{}, func() time.Time { return r.now }, slog.New(slog.NewTextHandler(r.logs, nil)))
	day := utcDay(2026, 6, 9)

	if err := r.obs.PageIngested(context.Background(), windowQuery(day), pageOf()); err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(r.logs.String(), "dispatch") {
		t.Fatalf("log = %q", r.logs.String())
	}
}

type failingDispatcher struct{}

func (failingDispatcher) Dispatch(context.Context) (appevents.Result, error) {
	return appevents.Result{}, errors.New("boom")
}
