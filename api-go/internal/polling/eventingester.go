package polling

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
)

// Event kinds written to application_event; they match the table's CHECK constraint.
const (
	EventNewApplication = "new_application"
	EventDecision       = "decision"
)

// ApplicationEvent is one outbox row: something worth alerting on happened to an application.
type ApplicationEvent struct {
	UID           string
	AuthorityCode string
	Kind          string
	EventDate     *time.Time
}

// ingestUnit is one transaction's worth of ingest work.
type ingestUnit interface {
	GetByUID(ctx context.Context, uid, authorityCode string) (applications.PlanningApplication, bool, error)
	Upsert(ctx context.Context, a applications.PlanningApplication) error
	InsertEvent(ctx context.Context, e ApplicationEvent) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type ingestUnitOpener interface {
	Begin(ctx context.Context) (ingestUnit, error)
}

// EventIngester upserts a planning application and records outbox events in a
// single transaction. It has no notification dependencies: a dispatcher drains
// application_event separately.
type EventIngester struct {
	opener ingestUnitOpener
}

// NewEventIngester wires an EventIngester over opener.
func NewEventIngester(opener ingestUnitOpener) *EventIngester {
	return &EventIngester{opener: opener}
}

// Ingest writes app and its events atomically. An application whose business
// and silent fields are unchanged writes nothing. Only a first sight
// (new_application, plus decision when already decided) or a move from a
// non-decision to a decision state (decision) creates an event.
func (i *EventIngester) Ingest(ctx context.Context, app applications.PlanningApplication) (err error) {
	unit, err := i.opener.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin ingest: %w", err)
	}
	defer func() {
		if err != nil {
			if rbErr := unit.Rollback(ctx); rbErr != nil {
				err = errors.Join(err, rbErr)
			}
		}
	}()

	authorityCode := strconv.Itoa(app.AreaID)
	existing, found, err := unit.GetByUID(ctx, app.UID, authorityCode)
	if err != nil {
		return err
	}
	if found && existing.HasSameBusinessFieldsAs(app) && existing.HasSameSilentFieldsAs(app) {
		return unit.Rollback(ctx)
	}

	if err = unit.Upsert(ctx, app); err != nil {
		return err
	}

	var events []ApplicationEvent
	if !found {
		events = append(events, ApplicationEvent{app.UID, authorityCode, EventNewApplication, app.StartDate})
	}
	var oldState *string
	if found {
		oldState = existing.AppState
	}
	if isDecisionState(app.AppState) && !isDecisionState(oldState) {
		events = append(events, ApplicationEvent{app.UID, authorityCode, EventDecision, app.DecidedDate})
	}
	for _, e := range events {
		if err = unit.InsertEvent(ctx, e); err != nil {
			return err
		}
	}
	if err = unit.Commit(ctx); err != nil {
		return fmt.Errorf("commit ingest: %w", err)
	}
	return nil
}

type pgIngestOpener struct{ pool *pgxpool.Pool }

// NewPostgresEventIngester wires an EventIngester over pool.
func NewPostgresEventIngester(pool *pgxpool.Pool) *EventIngester {
	return NewEventIngester(pgIngestOpener{pool: pool})
}

func (o pgIngestOpener) Begin(ctx context.Context) (ingestUnit, error) {
	tx, err := o.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &pgIngestUnit{PostgresStore: applications.NewPostgresStore(tx), tx: tx}, nil
}

type pgIngestUnit struct {
	*applications.PostgresStore
	tx pgx.Tx
}

const insertEventQuery = "INSERT INTO application_event (uid, authority_code, kind, event_date) VALUES ($1, $2, $3, $4)"

func (u *pgIngestUnit) InsertEvent(ctx context.Context, e ApplicationEvent) error {
	if _, err := u.tx.Exec(ctx, insertEventQuery, e.UID, e.AuthorityCode, e.Kind, e.EventDate); err != nil {
		return fmt.Errorf("insert application_event %q: %w", e.UID, err)
	}
	return nil
}

func (u *pgIngestUnit) Commit(ctx context.Context) error { return u.tx.Commit(ctx) }

// Rollback is a no-op after Commit, so it is safe on every exit path.
func (u *pgIngestUnit) Rollback(ctx context.Context) error {
	if err := u.tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return err
	}
	return nil
}
