package pgbackup

import (
	"context"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGSource opens snapshots on a pgx pool.
type PGSource struct {
	pool *pgxpool.Pool
}

// NewPGSource builds a PGSource over pool.
func NewPGSource(pool *pgxpool.Pool) *PGSource { return &PGSource{pool: pool} }

// Begin starts one REPEATABLE READ, READ ONLY transaction, so every table
// comes from the same snapshot.
func (p *PGSource) Begin(ctx context.Context) (Snapshot, error) {
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("begin repeatable read tx: %w", err)
	}
	return &pgSnapshot{tx: tx}, nil
}

type pgSnapshot struct {
	tx pgx.Tx
}

func (s *pgSnapshot) SchemaVersion(ctx context.Context) (int64, error) {
	var v int64
	err := s.tx.QueryRow(ctx, "SELECT COALESCE(max(version_id), 0) FROM goose_db_version WHERE is_applied").Scan(&v)
	return v, err
}

// CopyTable streams the table as CSV. Table names and keys are package
// constants, never user input.
func (s *pgSnapshot) CopyTable(ctx context.Context, w io.Writer, t Table) (int64, error) {
	sql := fmt.Sprintf("COPY (SELECT * FROM %s ORDER BY %s) TO STDOUT WITH (FORMAT csv, HEADER)", t.Name, t.OrderBy)
	tag, err := s.tx.Conn().PgConn().CopyTo(ctx, w, sql)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s *pgSnapshot) Close(ctx context.Context) error { return s.tx.Rollback(ctx) }
