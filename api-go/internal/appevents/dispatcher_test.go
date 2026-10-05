package appevents

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
)

var london = mustLocation("Europe/London")

func mustLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

func date(m time.Month, d int) *time.Time {
	t := time.Date(2026, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

type fakeStore struct {
	events []Event
	status map[int64]string
	err    error
}

func newFakeStore(events ...Event) *fakeStore {
	s := &fakeStore{events: events, status: map[int64]string{}}
	for _, e := range events {
		s.status[e.ID] = StatusPending
	}
	return s
}

func (s *fakeStore) Pending(context.Context) ([]Event, error) {
	if s.err != nil {
		return nil, s.err
	}
	var out []Event
	for _, e := range s.events {
		if s.status[e.ID] == StatusPending {
			out = append(out, e)
		}
	}
	return out, nil
}

func (s *fakeStore) MarkStale(_ context.Context, ids []int64, _ time.Time) error {
	for _, id := range ids {
		s.status[id] = StatusStale
	}
	return nil
}

func (s *fakeStore) MarkSent(_ context.Context, id int64, _ time.Time) error {
	s.status[id] = StatusSent
	return nil
}

func (s *fakeStore) CountActiveSince(_ context.Context, since time.Time) (int, error) {
	n := 0
	for _, e := range s.events {
		if s.status[e.ID] != StatusStale && !e.DetectedAt.Before(since) {
			n++
		}
	}
	return n, nil
}

type fakeApps struct {
	byUID map[string]applications.PlanningApplication
}

func (a fakeApps) GetByUID(_ context.Context, uid, _ string) (applications.PlanningApplication, bool, error) {
	app, ok := a.byUID[uid]
	return app, ok, nil
}

type fakeFanOut struct {
	calls []string
	times []time.Time
	err   error
}

func (f *fakeFanOut) Dispatch(_ context.Context, app applications.PlanningApplication) error {
	f.calls = append(f.calls, "decision:"+app.UID)
	return f.err
}

func (f *fakeFanOut) EnqueueForApplication(_ context.Context, app applications.PlanningApplication, at time.Time) error {
	f.calls = append(f.calls, "enqueue:"+app.UID)
	f.times = append(f.times, at)
	return f.err
}

type fakeFlusher struct{ n int }

func (f *fakeFlusher) Flush(context.Context) error { f.n++; return nil }

type harness struct {
	store   *fakeStore
	fan     *fakeFanOut
	flusher *fakeFlusher
	d       *Dispatcher
}

func newHarness(now time.Time, cfg Config, events ...Event) *harness {
	apps := fakeApps{byUID: map[string]applications.PlanningApplication{}}
	for _, e := range events {
		apps.byUID[e.UID] = applications.PlanningApplication{UID: e.UID}
	}
	h := &harness{store: newFakeStore(events...), fan: &fakeFanOut{}, flusher: &fakeFlusher{}}
	h.d = NewDispatcher(h.store, apps, h.fan, h.fan, h.flusher, cfg, func() time.Time { return now }, slog.New(slog.DiscardHandler))
	return h
}

func at(h, m int) time.Time { return time.Date(2026, 6, 15, h, m, 0, 0, london) }

func TestDispatch_QuietHours(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		now   time.Time
		quiet bool
	}{
		{"before start", at(21, 59), false},
		{"at start", at(22, 0), true},
		{"midnight", at(0, 0), true},
		{"just before end", at(6, 59), true},
		{"at end", at(7, 0), false},
		{"midday", at(12, 0), false},
		{"utc input converted to london summer time", time.Date(2026, 6, 15, 21, 30, 0, 0, time.UTC), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ev := Event{ID: 1, UID: "a", Kind: KindNewApplication, EventDate: date(6, 15), DetectedAt: tc.now}
			h := newHarness(tc.now, DefaultConfig(), ev)
			res, err := h.d.Dispatch(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if res.Quiet != tc.quiet {
				t.Errorf("Quiet = %v, want %v", res.Quiet, tc.quiet)
			}
			wantStatus, wantCalls := StatusSent, 1
			if tc.quiet {
				wantStatus, wantCalls = StatusPending, 0
			}
			if h.store.status[1] != wantStatus || len(h.fan.calls) != wantCalls {
				t.Errorf("status = %s, calls = %v", h.store.status[1], h.fan.calls)
			}
		})
	}
}

func TestDispatch_NonWrappingQuietWindow(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	cfg.QuietStart, cfg.QuietEnd = 13*time.Hour, 14*time.Hour
	h := newHarness(at(13, 30), cfg)
	res, err := h.d.Dispatch(context.Background())
	if err != nil || !res.Quiet {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
}

func TestDispatch_AgeFilter(t *testing.T) {
	t.Parallel()
	now := at(12, 0)
	tests := []struct {
		name       string
		eventDate  *time.Time
		detectedAt time.Time
		want       string
	}{
		{"14 days old fans out", date(6, 1), now, StatusSent},
		{"15 days old is stale", date(5, 31), now, StatusStale},
		{"today fans out", date(6, 15), now, StatusSent},
		{"null date falls back to detected_at, 14 days", nil, now.AddDate(0, 0, -14), StatusSent},
		{"null date falls back to detected_at, 15 days", nil, now.AddDate(0, 0, -15), StatusStale},
		{"null date uses london date of detected_at", nil, time.Date(2026, 5, 31, 23, 30, 0, 0, time.UTC), StatusSent},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ev := Event{ID: 1, UID: "a", Kind: KindNewApplication, EventDate: tc.eventDate, DetectedAt: tc.detectedAt}
			h := newHarness(now, DefaultConfig(), ev)
			res, err := h.d.Dispatch(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if h.store.status[1] != tc.want {
				t.Errorf("status = %s, want %s", h.store.status[1], tc.want)
			}
			wantCalls := 0
			if tc.want == StatusSent {
				wantCalls = 1
			}
			if len(h.fan.calls) != wantCalls {
				t.Errorf("calls = %v", h.fan.calls)
			}
			if res.Surge {
				t.Error("unexpected surge")
			}
		})
	}
}

func TestDispatch_Surge(t *testing.T) {
	t.Parallel()
	now := at(12, 0)
	build := func(n int, threshold int) *harness {
		var evs []Event
		for i := 1; i <= n; i++ {
			evs = append(evs, Event{ID: int64(i), UID: "u", Kind: KindNewApplication, EventDate: date(6, 15), DetectedAt: now.Add(-time.Hour)})
		}
		cfg := DefaultConfig()
		cfg.SurgeThreshold = threshold
		return newHarness(now, cfg, evs...)
	}
	tests := []struct {
		name  string
		n     int
		surge bool
	}{
		{"below threshold", 2, false},
		{"at threshold", 3, false},
		{"above threshold", 4, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := build(tc.n, 3)
			res, err := h.d.Dispatch(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if res.Surge != tc.surge {
				t.Fatalf("Surge = %v, want %v", res.Surge, tc.surge)
			}
			for id, st := range h.store.status {
				if tc.surge && st != StatusStale {
					t.Errorf("event %d = %s, want stale", id, st)
				}
				if !tc.surge && st != StatusSent {
					t.Errorf("event %d = %s, want sent", id, st)
				}
			}
			if tc.surge && len(h.fan.calls) != 0 {
				t.Errorf("surge fanned out: %v", h.fan.calls)
			}
		})
	}
}

func TestDispatch_StaleEventsDoNotTripSurge(t *testing.T) {
	t.Parallel()
	now := at(12, 0)
	var evs []Event
	for i := 1; i <= 5; i++ {
		evs = append(evs, Event{ID: int64(i), UID: "u", Kind: KindNewApplication, EventDate: date(1, 1), DetectedAt: now.Add(-time.Hour)})
	}
	cfg := DefaultConfig()
	cfg.SurgeThreshold = 3
	h := newHarness(now, cfg, evs...)
	res, err := h.d.Dispatch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Surge || res.Stale != 5 {
		t.Errorf("res = %+v", res)
	}
}

func TestDispatch_OrderingAndMapping(t *testing.T) {
	t.Parallel()
	now := at(12, 0)
	d1, d2, d3 := now.Add(-3*time.Hour), now.Add(-2*time.Hour), now.Add(-time.Hour)
	h := newHarness(now, DefaultConfig(),
		Event{ID: 1, UID: "a", Kind: KindNewApplication, EventDate: date(6, 15), DetectedAt: d1},
		Event{ID: 2, UID: "b", Kind: KindDecision, EventDate: date(6, 15), DetectedAt: d2},
		Event{ID: 3, UID: "c", Kind: KindNewApplication, EventDate: date(6, 15), DetectedAt: d3},
	)
	res, err := h.d.Dispatch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"enqueue:a", "decision:b", "enqueue:b", "enqueue:c"}
	if len(h.fan.calls) != len(want) {
		t.Fatalf("calls = %v", h.fan.calls)
	}
	for i := range want {
		if h.fan.calls[i] != want[i] {
			t.Fatalf("calls = %v, want %v", h.fan.calls, want)
		}
	}
	if !h.fan.times[0].Equal(d1) || !h.fan.times[1].Equal(d2) || !h.fan.times[2].Equal(d3) {
		t.Errorf("detectedAt not passed through: %v", h.fan.times)
	}
	if res.Sent != 3 {
		t.Errorf("Sent = %d", res.Sent)
	}
}

func TestDispatch_MissingApplicationMarkedStale(t *testing.T) {
	t.Parallel()
	now := at(12, 0)
	h := newHarness(now, DefaultConfig(), Event{ID: 1, UID: "a", Kind: KindNewApplication, EventDate: date(6, 15), DetectedAt: now})
	h.d.apps = fakeApps{byUID: map[string]applications.PlanningApplication{}}
	if _, err := h.d.Dispatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if h.store.status[1] != StatusStale || len(h.fan.calls) != 0 {
		t.Errorf("status = %s, calls = %v", h.store.status[1], h.fan.calls)
	}
}

func TestDispatch_FanOutErrorLeavesEventPending(t *testing.T) {
	t.Parallel()
	now := at(12, 0)
	h := newHarness(now, DefaultConfig(), Event{ID: 1, UID: "a", Kind: KindNewApplication, EventDate: date(6, 15), DetectedAt: now})
	boom := errors.New("boom")
	h.fan.err = boom
	if _, err := h.d.Dispatch(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if h.store.status[1] != StatusPending {
		t.Errorf("status = %s", h.store.status[1])
	}
}

func TestFlush_CallsCoalescerOnce(t *testing.T) {
	t.Parallel()
	h := newHarness(at(12, 0), DefaultConfig())
	if err := h.d.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if h.flusher.n != 1 {
		t.Errorf("Flush calls = %d", h.flusher.n)
	}
}

func TestParseTimeOfDay(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"22:00", 22 * time.Hour, false},
		{"07:30", 7*time.Hour + 30*time.Minute, false},
		{"24:00", 0, true},
		{"7", 0, true},
		{"aa:bb", 0, true},
	}
	for _, tc := range tests {
		got, err := ParseTimeOfDay(tc.in)
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("ParseTimeOfDay(%q) = %v, %v", tc.in, got, err)
		}
	}
}
