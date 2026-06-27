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

// TestBuildDormant_FinderIsFlagSelectedPostgresAdminStore proves the
// dormant-cleanup FINDER (not just the deleters) is flag-selected to Postgres
// when the full-cutover stores are present. This is the GDPR-retention guard for
// tc-hpd2.13: post-cutover the Cosmos Users container is dark, so a finder still
// hardcoded to Cosmos would scan an empty container, find zero dormant accounts,
// and silently stop erasing inactive users.
//
// The test injects a recording querier through pgStores.profileAdmin, then drives
// the real handler: handler.Run calls finder.Dormant first, which (for the
// Postgres admin store) issues the last_active_at_epoch scan through the fake.
// The sentinel error proves the finder routed through Postgres; if the finder
// were still the Cosmos admin store the fake would never be touched and the error
// would be a Cosmos failure instead.
func TestBuildDormant_FinderIsFlagSelectedPostgresAdminStore(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("pg dormant scan reached")
	q := &recordingQuerier{queryErr: sentinel}
	pg := &pgStores{profileAdmin: profiles.NewPostgresAdminStore(q)}

	// A non-empty endpoint clears buildDormant's Cosmos-config guard; the azcosmos
	// client is built lazily so no network call is made. The Cosmos deleters are
	// constructed but never reached — Run aborts on the finder's error first.
	cfg := platform.Config{
		CosmosEndpoint: "https://example.documents.azure.com:443/",
		CosmosDatabase: "db",
	}

	handler, err := buildDormant(cfg, testRegistry(), backendPostgres, pg, discardLogger())
	if err != nil {
		t.Fatalf("buildDormant: %v", err)
	}
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
