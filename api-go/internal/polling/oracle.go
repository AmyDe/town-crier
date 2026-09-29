package polling

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

const (
	oracleMaxPages = 60

	ReasonLate         = "late"
	ReasonDateChanged  = "date_changed"
	ReasonPlanItGone   = "planit_deleted"
	ReasonOracleMissed = "miss"
)

// WindowMembers is the record set of a window's latest full read and when it
// was made.
type WindowMembers struct {
	ReadAt time.Time
	Keys   map[AppKey]struct{}
}

// OracleDiff is one poll_oracle_diff row. Reason is empty until classified.
type OracleDiff struct {
	Axis    planit.Axis
	Day     time.Time
	UID     string
	AreaID  int
	FoundAt time.Time
	Reason  string
}

type oracleMemberStore interface {
	Members(ctx context.Context, axis planit.Axis, from time.Time) (map[time.Time]WindowMembers, error)
}

type oracleDiffStore interface {
	Insert(ctx context.Context, rows []OracleDiff) error
	Unclassified(ctx context.Context, axis planit.Axis) ([]OracleDiff, error)
	Classify(ctx context.Context, d OracleDiff, reason string) error
}

type oracleEventStore interface {
	Record(ctx context.Context, e PollEvent) error
	DoneSince(ctx context.Context, axis planit.Axis, since time.Time) (bool, error)
}

// Oracle is the dev-only canary: one wide read per axis over the coverage
// range, compared with the latest complete window reads. It never ingests.
type Oracle struct {
	fetch   windowPageFetcher
	members oracleMemberStore
	diffs   oracleDiffStore
	events  oracleEventStore
	now     func() time.Time
	log     *slog.Logger
}

// NewOracle wires an Oracle.
func NewOracle(fetch windowPageFetcher, members oracleMemberStore, diffs oracleDiffStore, events oracleEventStore, now func() time.Time, log *slog.Logger) *Oracle {
	return &Oracle{fetch: fetch, members: members, diffs: diffs, events: events, now: now, log: log}
}

// Run makes the wide read for each axis not yet done since the current budget
// day began. It returns the first fetch or store error; a PlanIt limit is also
// reported through the outcome's Stop.
func (o *Oracle) Run(ctx context.Context, now time.Time) (OracleOutcome, error) {
	res, err := o.run(ctx, now)
	return res, err
}

func (o *Oracle) run(ctx context.Context, now time.Time) (OracleOutcome, error) {
	res := OracleOutcome{Classified: map[string]int{}}
	nightStart, _ := BudgetDay(now)
	for _, axis := range [...]planit.Axis{planit.AxisStart, planit.AxisDecided} {
		done, err := o.events.DoneSince(ctx, axis, nightStart)
		if err != nil {
			return res, fmt.Errorf("oracle done check: %w", err)
		}
		if done {
			continue
		}
		if err := o.runAxis(ctx, axis, &res); err != nil {
			return res, err
		}
	}
	return res, nil
}

func (o *Oracle) runAxis(ctx context.Context, axis planit.Axis, res *OracleOutcome) error {
	wideStart := o.now()
	local := wideStart.In(budgetLocation)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	from := today.AddDate(0, 0, -coverageBandMaxAge)

	wide, err := o.wideRead(ctx, axis, from, today, res)
	if err != nil {
		return err
	}
	members, err := o.members.Members(ctx, axis, from)
	if err != nil {
		return fmt.Errorf("load poll_window_member: %w", err)
	}
	if err := o.classifyPending(ctx, axis, wide, members, res); err != nil {
		return err
	}
	if err := o.findDiffs(ctx, axis, wide, members, wideStart, today, res); err != nil {
		return err
	}
	if err := o.events.Record(ctx, PollEvent{Kind: EventOracleDone, At: wideStart, Axis: axis, Detail: len(wide)}); err != nil {
		return fmt.Errorf("record oracle_done: %w", err)
	}
	o.log.InfoContext(ctx, "poll.oracle_night",
		slog.String("axis", axisName(axis)), slog.Int("wide_records", len(wide)),
		slog.Int("new_diffs", res.NewDiffs), slog.Any("classified", res.Classified))
	return nil
}

func (o *Oracle) wideRead(ctx context.Context, axis planit.Axis, from, to time.Time, res *OracleOutcome) (map[AppKey]time.Time, error) {
	work := planit.WorkOracleStart
	if axis == planit.AxisDecided {
		work = planit.WorkOracleDecided
	}
	q := planit.WindowQuery{Work: work, Axis: axis, From: from, To: to}
	wide := map[AppKey]time.Time{}
	for pages := 0; ; {
		p, err := o.fetch.FetchPage(ctx, q)
		if err != nil {
			res.Stop = stopReasonFor(err)
			return nil, fmt.Errorf("oracle wide read: %w", err)
		}
		pages++
		res.Pages++
		res.WideRecords += len(p.Applications)
		for _, app := range p.Applications {
			if d := axisDate(app, axis); d != nil {
				wide[AppKey{UID: app.UID, AreaID: app.AreaID}] = dateOnly(*d)
			}
		}
		if !p.HasMorePages || len(p.Applications) == 0 {
			return wide, nil
		}
		if pages >= oracleMaxPages {
			res.Stop = StopError
			return nil, fmt.Errorf("oracle wide read: more than %d pages", oracleMaxPages)
		}
		q.Index = p.From + len(p.Applications)
	}
}

func (o *Oracle) classifyPending(ctx context.Context, axis planit.Axis, wide map[AppKey]time.Time, members map[time.Time]WindowMembers, res *OracleOutcome) error {
	pending, err := o.diffs.Unclassified(ctx, axis)
	if err != nil {
		return fmt.Errorf("load unclassified oracle diffs: %w", err)
	}
	for _, d := range pending {
		m, ok := members[d.Day]
		if !ok || !m.ReadAt.After(d.FoundAt) {
			continue
		}
		key := AppKey{UID: d.UID, AreaID: d.AreaID}
		wideDay, inWide := wide[key]
		var reason string
		switch _, inWindow := m.Keys[key]; {
		case inWindow:
			reason = ReasonLate
		case inWide && wideDay.Equal(d.Day):
			reason = ReasonOracleMissed
		case inWide:
			reason = ReasonDateChanged
		default:
			reason = ReasonPlanItGone
		}
		if err := o.diffs.Classify(ctx, d, reason); err != nil {
			return fmt.Errorf("classify oracle diff: %w", err)
		}
		res.Classified[reason]++
		if reason == ReasonOracleMissed {
			o.log.ErrorContext(ctx, "poll.oracle_miss", slog.String("uid", d.UID), slog.Int("area_id", d.AreaID),
				slog.String("axis", axisName(axis)), slog.String("day", d.Day.Format(time.DateOnly)))
		}
	}
	return nil
}

func (o *Oracle) findDiffs(ctx context.Context, axis planit.Axis, wide map[AppKey]time.Time, members map[time.Time]WindowMembers, wideStart, today time.Time, res *OracleOutcome) error {
	oldest := today.AddDate(0, 0, -alertBandMaxAge)
	var rows []OracleDiff
	for key, day := range wide {
		if day.Before(oldest) || day.After(today) {
			continue
		}
		res.InBand++
		m, ok := members[day]
		if !ok || !m.ReadAt.Before(wideStart) {
			continue
		}
		if _, in := m.Keys[key]; in {
			continue
		}
		rows = append(rows, OracleDiff{Axis: axis, Day: day, UID: key.UID, AreaID: key.AreaID, FoundAt: wideStart})
	}
	if len(rows) == 0 {
		return nil
	}
	if err := o.diffs.Insert(ctx, rows); err != nil {
		return fmt.Errorf("insert oracle diffs: %w", err)
	}
	res.NewDiffs += len(rows)
	return nil
}
