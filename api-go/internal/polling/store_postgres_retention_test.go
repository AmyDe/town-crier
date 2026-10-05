package polling

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type recordingExec struct {
	sqls []string
	args [][]any
	err  error
}

func (r *recordingExec) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	r.sqls = append(r.sqls, sql)
	r.args = append(r.args, args)
	return pgconn.NewCommandTag("DELETE 2"), r.err
}

func (r *recordingExec) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, nil }
func (r *recordingExec) QueryRow(context.Context, string, ...any) pgx.Row        { return nil }

func TestPostgresRetentionStore_PurgeUsesEachTablesCutoff(t *testing.T) {
	t.Parallel()

	db := &recordingExec{}
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)

	got, err := NewPostgresRetentionStore(db).Purge(context.Background(), now)
	if err != nil {
		t.Fatalf("Purge: %v", err)
	}

	wantCutoffs := map[string]time.Time{
		"planit_call":        now.AddDate(0, 0, -90),
		"poll_event":         now.AddDate(0, 0, -90),
		"application_event":  now.AddDate(0, 0, -90),
		"poll_oracle_diff":   now.AddDate(0, 0, -90),
		"delta_seen":         now.AddDate(0, 0, -14),
		"poll_window_member": now.AddDate(0, 0, -120),
		"poll_window":        now.AddDate(0, 0, -120),
	}
	if len(got) != len(wantCutoffs) {
		t.Fatalf("result tables = %v, want %d tables", got, len(wantCutoffs))
	}
	for table, cutoff := range wantCutoffs {
		if got[table] != 2 {
			t.Errorf("%s count = %d, want 2", table, got[table])
		}
		found := false
		for i, sql := range db.sqls {
			if containsTable(sql, table) {
				found = true
				if c, ok := db.args[i][0].(time.Time); !ok || !c.Equal(cutoff) {
					t.Errorf("%s cutoff = %v, want %v", table, db.args[i][0], cutoff)
				}
			}
		}
		if !found {
			t.Errorf("no DELETE issued for %s", table)
		}
	}
}

func containsTable(sql, table string) bool {
	return len(sql) > len("DELETE FROM ")+len(table) && sql[:len("DELETE FROM ")+len(table)+1] == "DELETE FROM "+table+" "
}

func TestPostgresRetentionStore_PurgeStopsOnError(t *testing.T) {
	t.Parallel()

	boom := errors.New("connection reset")
	db := &recordingExec{err: boom}

	if _, err := NewPostgresRetentionStore(db).Purge(context.Background(), time.Now()); !errors.Is(err, boom) {
		t.Fatalf("Purge error = %v, want %v", err, boom)
	}
	if len(db.sqls) != 1 {
		t.Errorf("issued %d deletes after the first error, want 1", len(db.sqls))
	}
}
