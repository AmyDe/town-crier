package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// recordingQuerier is a hand-written profiles.querier double. Its Query records
// the call and returns a sentinel error so the caller's Dormant scan surfaces
// it; Exec/QueryRow panic so any unintended use is caught. Returning (nil,
// sentinel) avoids implementing the full pgx.Rows interface — the dormant
// finder's error path is all this test needs.
type recordingQuerier struct {
	queryCalls int
	lastSQL    string
	queryErr   error
}

func (q *recordingQuerier) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	q.queryCalls++
	q.lastSQL = sql
	return nil, q.queryErr
}

func (q *recordingQuerier) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	panic("recordingQuerier.Exec not expected in this test")
}

func (q *recordingQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	panic("recordingQuerier.QueryRow not expected in this test")
}

// TestBuildDormant_FinderIsPostgresAdminStore proves the dormant-cleanup FINDER
// (not just the deleters) routes through the Postgres admin store. This is the
// GDPR-retention guard for tc-hpd2.13: the finder must scan the live Postgres
// users so inactive accounts keep being erased.
//
// The test injects a recording querier through stores.profileAdmin, then drives
// the real handler: handler.Run calls finder.Dormant first, which issues the
// last_active_at_epoch scan through the fake. The sentinel error proves the
// finder routed through Postgres. The other deleters are constructed from nil
// store fields but are never reached — Run aborts on the finder's error first.
func TestBuildDormant_FinderIsPostgresAdminStore(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("pg dormant scan reached")
	q := &recordingQuerier{queryErr: sentinel}
	st := &stores{profileAdmin: profiles.NewPostgresAdminStore(q)}

	handler := buildDormant(platform.Config{}, st, discardLogger())
	if handler == nil {
		t.Fatal("buildDormant returned a nil handler")
	}

	_, runErr := handler.Run(context.Background())
	if !errors.Is(runErr, sentinel) {
		t.Fatalf("dormant finder did not route through the Postgres admin store: %v", runErr)
	}
	if q.queryCalls != 1 {
		t.Fatalf("Postgres admin store Query calls = %d, want 1", q.queryCalls)
	}
	if !strings.Contains(q.lastSQL, "last_active_at_epoch") {
		t.Fatalf("finder query was not the Postgres Dormant scan: %q", q.lastSQL)
	}
}
