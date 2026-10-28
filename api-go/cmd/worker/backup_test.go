package main

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/pgbackup"
)

func TestBuildBackup_FailsClosedOnMissingConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		env  map[string]string
	}{
		{"nothing set", map[string]string{}},
		{"no container", map[string]string{"BACKUP_BLOB_URL": "https://x.blob.core.windows.net"}},
		{"no url", map[string]string{"BACKUP_CONTAINER": "pg-backups"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			getenv := func(k string) string { return tt.env[k] }
			if _, err := buildBackup(getenv, nil, slog.Default()); !errors.Is(err, pgbackup.ErrMissingBlobConfig) {
				t.Fatalf("error = %v, want ErrMissingBlobConfig", err)
			}
		})
	}
}

type fakeExporter struct {
	res pgbackup.Result
	err error
}

func (f fakeExporter) Run(context.Context) (pgbackup.Result, error) { return f.res, f.err }

func TestBackupRunner_MapsResultToTableStats(t *testing.T) {
	t.Parallel()
	r := backupRunner{exporter: fakeExporter{res: pgbackup.Result{Tables: []pgbackup.TableResult{{Name: "users", Rows: 2, Bytes: 9}}}}}

	got, err := r.Run(context.Background())

	if err != nil || len(got) != 1 || got[0].Name != "users" || got[0].Rows != 2 || got[0].Bytes != 9 {
		t.Errorf("got %+v, err %v", got, err)
	}
}
