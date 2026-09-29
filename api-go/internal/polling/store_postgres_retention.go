package polling

import (
	"context"
	"fmt"
	"time"
)

const (
	auditRetentionDays  = 90
	deltaSeenRetention  = 14
	windowStateRetained = 120
)

type retentionRule struct {
	table string
	sql   string
	days  int
}

var retentionRules = [...]retentionRule{
	{"planit_call", "DELETE FROM planit_call WHERE at < $1", auditRetentionDays},
	{"poll_event", "DELETE FROM poll_event WHERE at < $1", auditRetentionDays},
	{"application_event", "DELETE FROM application_event WHERE detected_at < $1", auditRetentionDays},
	{"poll_oracle_diff", "DELETE FROM poll_oracle_diff WHERE found_at < $1", auditRetentionDays},
	{"delta_seen", "DELETE FROM delta_seen WHERE seen_at < $1", deltaSeenRetention},
	{"poll_window_member", "DELETE FROM poll_window_member WHERE day < $1::date", windowStateRetained},
	{"poll_window", "DELETE FROM poll_window WHERE day < $1::date", windowStateRetained},
}

// PostgresRetentionStore deletes stale rows from the polling tables.
type PostgresRetentionStore struct {
	db querier
}

// NewPostgresRetentionStore wires the store over db.
func NewPostgresRetentionStore(db querier) *PostgresRetentionStore {
	return &PostgresRetentionStore{db: db}
}

// Purge deletes rows older than each table's retention relative to now: the
// call log, events and oracle diffs after 90 days, delta_seen after 14 days and
// poll_window and its members after 120 days. It returns the rows deleted per
// table and stops at the first error.
func (s *PostgresRetentionStore) Purge(ctx context.Context, now time.Time) (map[string]int64, error) {
	deleted := make(map[string]int64, len(retentionRules))
	for _, r := range retentionRules {
		tag, err := s.db.Exec(ctx, r.sql, now.AddDate(0, 0, -r.days))
		if err != nil {
			return deleted, fmt.Errorf("purge %s: %w", r.table, err)
		}
		deleted[r.table] = tag.RowsAffected()
	}
	return deleted, nil
}
