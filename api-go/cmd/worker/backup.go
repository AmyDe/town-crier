package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/pgbackup"
	"github.com/AmyDe/town-crier/api-go/internal/worker"
)

type backupExporter interface {
	Run(ctx context.Context) (pgbackup.Result, error)
}

type backupRunner struct{ exporter backupExporter }

func (b backupRunner) Run(ctx context.Context) ([]worker.BackupTableStat, error) {
	res, err := b.exporter.Run(ctx)
	if err != nil {
		return nil, err
	}
	stats := make([]worker.BackupTableStat, len(res.Tables))
	for i, t := range res.Tables {
		stats[i] = worker.BackupTableStat{Name: t.Name, Rows: t.Rows, Bytes: t.Bytes}
	}
	return stats, nil
}

// buildBackup wires the pg-backup runner from BACKUP_BLOB_URL, BACKUP_CONTAINER
// and AZURE_CLIENT_ID. A missing URL or container is an error, so the mode
// fails closed.
func buildBackup(getenv func(string) string, pool *pgxpool.Pool, logger *slog.Logger) (worker.BackupRunner, error) {
	sink, err := pgbackup.NewBlobSink(getenv("BACKUP_BLOB_URL"), getenv("BACKUP_CONTAINER"), getenv("AZURE_CLIENT_ID"))
	if err != nil {
		return nil, err
	}
	return backupRunner{exporter: pgbackup.New(pgbackup.NewPGSource(pool), sink, time.Now, logger)}, nil
}
