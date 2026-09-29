package polling

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
)

type fakeUnit struct {
	existing   *applications.PlanningApplication
	upserts    []applications.PlanningApplication
	events     []ApplicationEvent
	committed  bool
	rolledBack bool
	upsertErr  error
	eventErr   error
	commitErr  error
}

func (u *fakeUnit) GetByUID(_ context.Context, _, _ string) (applications.PlanningApplication, bool, error) {
	if u.existing == nil {
		return applications.PlanningApplication{}, false, nil
	}
	return *u.existing, true, nil
}

func (u *fakeUnit) Upsert(_ context.Context, a applications.PlanningApplication) error {
	u.upserts = append(u.upserts, a)
	return u.upsertErr
}

func (u *fakeUnit) InsertEvent(_ context.Context, e ApplicationEvent) error {
	u.events = append(u.events, e)
	return u.eventErr
}

func (u *fakeUnit) Commit(context.Context) error   { u.committed = true; return u.commitErr }
func (u *fakeUnit) Rollback(context.Context) error { u.rolledBack = true; return nil }

type fakeOpener struct{ unit *fakeUnit }

func (o fakeOpener) Begin(context.Context) (ingestUnit, error) { return o.unit, nil }

func evApp(state string, start, decided *time.Time) applications.PlanningApplication {
	a := testApp("24/1", 300, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))
	a.AppState = &state
	a.StartDate = start
	a.DecidedDate = decided
	return a
}

func day(d int) *time.Time {
	t := time.Date(2026, 6, d, 0, 0, 0, 0, time.UTC)
	return &t
}

func TestEventIngester_Ingest(t *testing.T) {
	t.Parallel()
	newApp := evApp("Undecided", day(1), nil)
	decidedNew := evApp("Permitted", day(1), day(5))
	oldUndecided := evApp("Undecided", day(1), nil)
	oldPermitted := evApp("Permitted", day(1), day(5))
	oldPermittedDesc := oldPermitted
	oldPermittedDesc.Description = "changed"
	toRejected := oldPermitted
	rej := "Rejected"
	toRejected.AppState = &rej
	silentOnly := oldUndecided
	ref := "ref-2"
	silentOnly.Reference = &ref
	descChange := oldUndecided
	descChange.Description = "other"
	toPermitted := evApp("Permitted", day(1), day(6))

	tests := []struct {
		name       string
		existing   *applications.PlanningApplication
		app        applications.PlanningApplication
		wantUpsert bool
		wantEvents []ApplicationEvent
	}{
		{"new undecided", nil, newApp, true, []ApplicationEvent{{"24/1/FUL", "300", EventNewApplication, day(1)}}},
		{"new decided", nil, decidedNew, true, []ApplicationEvent{
			{"24/1/FUL", "300", EventNewApplication, day(1)},
			{"24/1/FUL", "300", EventDecision, day(5)}}},
		{"unchanged", &oldUndecided, oldUndecided, false, nil},
		{"transition to decision", &oldUndecided, toPermitted, true, []ApplicationEvent{{"24/1/FUL", "300", EventDecision, day(6)}}},
		{"silent only", &oldUndecided, silentOnly, true, nil},
		{"non-decision field change", &oldUndecided, descChange, true, nil},
		{"decision to decision", &oldPermitted, toRejected, true, nil},
		{"decided field change", &oldPermitted, oldPermittedDesc, true, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			u := &fakeUnit{existing: tc.existing}
			ing := NewEventIngester(fakeOpener{u})
			if err := ing.Ingest(context.Background(), tc.app); err != nil {
				t.Fatal(err)
			}
			if (len(u.upserts) == 1) != tc.wantUpsert {
				t.Fatalf("upserts = %d", len(u.upserts))
			}
			if len(u.events) != len(tc.wantEvents) {
				t.Fatalf("events = %+v, want %+v", u.events, tc.wantEvents)
			}
			for i, w := range tc.wantEvents {
				g := u.events[i]
				if g.UID != w.UID || g.AuthorityCode != w.AuthorityCode || g.Kind != w.Kind || !g.EventDate.Equal(*w.EventDate) {
					t.Errorf("event %d = %+v, want %+v", i, g, w)
				}
			}
			if !u.committed && tc.wantUpsert {
				t.Errorf("committed=%v", u.committed)
			}
		})
	}
}

func TestEventIngester_Ingest_ErrorRollsBackAndSkipsCommit(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	for name, u := range map[string]*fakeUnit{
		"upsert": {upsertErr: boom},
		"event":  {eventErr: boom},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := NewEventIngester(fakeOpener{u}).Ingest(context.Background(), evApp("Permitted", day(1), day(2)))
			if !errors.Is(err, boom) {
				t.Fatalf("err = %v", err)
			}
			if u.committed || !u.rolledBack {
				t.Errorf("committed=%v rolledBack=%v", u.committed, u.rolledBack)
			}
		})
	}
}
