package polling

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

type fakeOracleMembers struct {
	byAxis map[planit.Axis]map[time.Time]WindowMembers
}

func (f *fakeOracleMembers) Members(_ context.Context, axis planit.Axis, _ time.Time) (map[time.Time]WindowMembers, error) {
	return f.byAxis[axis], nil
}

type fakeOracleDiffs struct {
	inserted   []OracleDiff
	pending    map[planit.Axis][]OracleDiff
	classified map[string]string
}

func (f *fakeOracleDiffs) Insert(_ context.Context, rows []OracleDiff) error {
	f.inserted = append(f.inserted, rows...)
	return nil
}

func (f *fakeOracleDiffs) Unclassified(_ context.Context, axis planit.Axis) ([]OracleDiff, error) {
	return f.pending[axis], nil
}

func (f *fakeOracleDiffs) Classify(_ context.Context, d OracleDiff, reason string) error {
	if f.classified == nil {
		f.classified = map[string]string{}
	}
	f.classified[d.UID] = reason
	return nil
}

type fakeOracleEvents struct {
	done    map[planit.Axis]time.Time
	written []PollEvent
}

func (f *fakeOracleEvents) Record(_ context.Context, e PollEvent) error {
	f.written = append(f.written, e)
	return nil
}

func (f *fakeOracleEvents) DoneSince(_ context.Context, axis planit.Axis, since time.Time) (bool, error) {
	at, ok := f.done[axis]
	return ok && !at.Before(since), nil
}

type oracleRig struct {
	fetch   *fakeWindowFetcher
	members *fakeOracleMembers
	diffs   *fakeOracleDiffs
	events  *fakeOracleEvents
	oracle  *Oracle
	now     time.Time
}

func newOracleRig(t *testing.T, pages map[int]planit.FetchPageResult) *oracleRig {
	t.Helper()
	r := &oracleRig{
		fetch:   &fakeWindowFetcher{pages: pages, errAt: map[int]error{}},
		members: &fakeOracleMembers{byAxis: map[planit.Axis]map[time.Time]WindowMembers{}},
		diffs:   &fakeOracleDiffs{},
		events:  &fakeOracleEvents{done: map[planit.Axis]time.Time{}},
		now:     londonAt(6, 11, 2, 0),
	}
	r.oracle = NewOracle(r.fetch, r.members, r.diffs, r.events, func() time.Time { return r.now }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return r
}

func keys(ks ...AppKey) map[AppKey]struct{} {
	m := map[AppKey]struct{}{}
	for _, k := range ks {
		m[k] = struct{}{}
	}
	return m
}

func startApp(uid string, day time.Time) applications.PlanningApplication {
	return applications.PlanningApplication{UID: uid, AreaID: 1, StartDate: &day}
}

func widePage(apps ...applications.PlanningApplication) map[int]planit.FetchPageResult {
	n := len(apps)
	return map[int]planit.FetchPageResult{0: {From: 0, Applications: apps, Total: &n}}
}

// windowRead is the member set of a window read an hour before the oracle.
func (r *oracleRig) windowRead(axis planit.Axis, day time.Time, ks ...AppKey) {
	if r.members.byAxis[axis] == nil {
		r.members.byAxis[axis] = map[time.Time]WindowMembers{}
	}
	r.members.byAxis[axis][day] = WindowMembers{ReadAt: r.now.Add(-time.Hour), Keys: keys(ks...)}
}

var (
	oracleDay   = utcDay(2026, 6, 9)
	oracleToday = utcDay(2026, 6, 11)
)

func TestOracle_WideReadShapeAndPaging(t *testing.T) {
	t.Parallel()
	total := 400
	r := newOracleRig(t, map[int]planit.FetchPageResult{
		0:   {From: 0, Applications: recs(1, 0, 300), Total: &total, HasMorePages: true},
		300: {From: 300, Applications: recs(1, 300, 100), Total: &total},
	})

	out, err := r.oracle.Run(context.Background(), r.now)

	if err != nil {
		t.Fatal(err)
	}
	if out.Pages != 4 || out.Stop != "" {
		t.Fatalf("out = %+v", out)
	}
	q := r.fetch.queries[0]
	if q.Work != planit.WorkOracleStart || q.Axis != planit.AxisStart || !q.From.Equal(oracleToday.AddDate(0, 0, -90)) || !q.To.Equal(oracleToday) || q.DifferentStart != nil {
		t.Fatalf("start query = %+v", q)
	}
	if got := r.fetch.queries[1].Index; got != 300 {
		t.Fatalf("second page index = %d", got)
	}
	if last := r.fetch.queries[len(r.fetch.queries)-1]; last.Work != planit.WorkOracleDecided || last.Axis != planit.AxisDecided {
		t.Fatalf("decided query = %+v", last)
	}
}

func TestOracle_RecordAbsentFromWindowReadIsADiff(t *testing.T) {
	t.Parallel()
	r := newOracleRig(t, widePage(
		startApp("present", oracleDay),
		startApp("absent", oracleDay),
		startApp("old", utcDay(2026, 5, 20)),
		startApp("nowindow", utcDay(2026, 6, 10)),
	))
	r.windowRead(planit.AxisStart, oracleDay, AppKey{"present", 1})

	out, err := r.oracle.Run(context.Background(), r.now)

	if err != nil {
		t.Fatal(err)
	}
	if len(r.diffs.inserted) != 1 {
		t.Fatalf("diffs = %+v", r.diffs.inserted)
	}
	d := r.diffs.inserted[0]
	if d.UID != "absent" || d.AreaID != 1 || d.Axis != planit.AxisStart || !d.Day.Equal(oracleDay) || !d.FoundAt.Equal(r.now) || d.Reason != "" {
		t.Fatalf("diff = %+v", d)
	}
	if out.WideRecords != 8 || out.NewDiffs != 1 {
		t.Fatalf("out = %+v", out)
	}
}

func TestOracle_WindowReadMadeAfterTheWideReadStartedIsNotComparedAgainst(t *testing.T) {
	t.Parallel()
	r := newOracleRig(t, widePage(startApp("absent", oracleDay)))
	r.members.byAxis[planit.AxisStart] = map[time.Time]WindowMembers{oracleDay: {ReadAt: r.now.Add(time.Minute), Keys: keys()}}

	if _, err := r.oracle.Run(context.Background(), r.now); err != nil {
		t.Fatal(err)
	}

	if len(r.diffs.inserted) != 0 {
		t.Fatalf("diffs = %+v", r.diffs.inserted)
	}
}

func TestOracle_ClassifiesEarlierDiffsAtTheNextNightsRead(t *testing.T) {
	t.Parallel()
	found := londonAt(6, 10, 2, 0)
	moved := utcDay(2026, 6, 3)
	tests := []struct {
		name    string
		members map[time.Time]WindowMembers
		wide    []applications.PlanningApplication
		want    string
	}{
		{"next window read returns it", map[time.Time]WindowMembers{oracleDay: {ReadAt: found.Add(24 * time.Hour), Keys: keys(AppKey{"x", 1})}}, nil, "late"},
		{"wide read returns it with another axis date", map[time.Time]WindowMembers{oracleDay: {ReadAt: found.Add(24 * time.Hour), Keys: keys()}}, []applications.PlanningApplication{startApp("x", moved)}, "date_changed"},
		{"neither read returns it", map[time.Time]WindowMembers{oracleDay: {ReadAt: found.Add(24 * time.Hour), Keys: keys()}}, nil, "planit_deleted"},
		{"wide read returns it in the window and the window read does not", map[time.Time]WindowMembers{oracleDay: {ReadAt: found.Add(24 * time.Hour), Keys: keys()}}, []applications.PlanningApplication{startApp("x", oracleDay)}, "miss"},
		{"no window read since the diff stays unclassified", map[time.Time]WindowMembers{oracleDay: {ReadAt: found.Add(-time.Hour), Keys: keys()}}, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newOracleRig(t, widePage(tt.wide...))
			r.members.byAxis[planit.AxisStart] = tt.members
			r.diffs.pending = map[planit.Axis][]OracleDiff{planit.AxisStart: {{Axis: planit.AxisStart, Day: oracleDay, UID: "x", AreaID: 1, FoundAt: found}}}

			out, err := r.oracle.Run(context.Background(), r.now)

			if err != nil {
				t.Fatal(err)
			}
			if got := r.diffs.classified["x"]; got != tt.want {
				t.Fatalf("reason = %q, want %q", got, tt.want)
			}
			if tt.want != "" && out.Classified[tt.want] != 1 {
				t.Fatalf("classified = %+v", out.Classified)
			}
		})
	}
}

func TestOracle_RunsOncePerNightPerAxis(t *testing.T) {
	t.Parallel()
	r := newOracleRig(t, widePage())
	r.events.done[planit.AxisStart] = londonAt(6, 10, 22, 0)
	r.events.done[planit.AxisDecided] = londonAt(6, 10, 17, 0)

	out, err := r.oracle.Run(context.Background(), r.now)

	if err != nil {
		t.Fatal(err)
	}
	if len(r.fetch.queries) != 1 || r.fetch.queries[0].Axis != planit.AxisDecided {
		t.Fatalf("queries = %+v", r.fetch.queries)
	}
	if out.Pages != 1 {
		t.Fatalf("out = %+v", out)
	}
	if len(r.events.written) != 1 || r.events.written[0].Kind != EventOracleDone || r.events.written[0].Axis != planit.AxisDecided {
		t.Fatalf("events = %+v", r.events.written)
	}
}

func TestOracle_FetchFailureStopsWithoutMarkingDone(t *testing.T) {
	t.Parallel()
	r := newOracleRig(t, widePage())
	r.fetch.errAt[0] = &planit.RateLimitError{}

	out, err := r.oracle.Run(context.Background(), r.now)

	if err == nil || out.Stop != StopRateLimited {
		t.Fatalf("out = %+v err = %v", out, err)
	}
	if len(r.events.written) != 0 || len(r.fetch.queries) != 1 {
		t.Fatalf("events = %+v queries = %d", r.events.written, len(r.fetch.queries))
	}
	if !errors.As(err, new(*planit.RateLimitError)) {
		t.Fatalf("err = %v", err)
	}
}
