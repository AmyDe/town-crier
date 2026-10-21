package indexnow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const stateKey = "indexnow_sitemap"

var _ StateStore = (*PostgresStateStore)(nil)

// PostgresStateStore keeps the last accepted sitemap set in seo_state.
type PostgresStateStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStateStore wires the store over pool.
func NewPostgresStateStore(pool *pgxpool.Pool) *PostgresStateStore {
	return &PostgresStateStore{pool: pool}
}

// Load returns the stored loc -> lastmod set, or false when none is stored.
func (s *PostgresStateStore) Load(ctx context.Context) (map[string]string, bool, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, "SELECT value FROM seo_state WHERE key = $1", stateKey).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("select seo_state: %w", err)
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, false, fmt.Errorf("decode seo_state %s: %w", stateKey, err)
	}
	return m, true, nil
}

// Save replaces the stored set.
func (s *PostgresStateStore) Save(ctx context.Context, current map[string]string) error {
	raw, err := json.Marshal(current)
	if err != nil {
		return fmt.Errorf("encode seo_state %s: %w", stateKey, err)
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO seo_state (key, value, updated_at) VALUES ($1, $2, now())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at`, stateKey, raw)
	if err != nil {
		return fmt.Errorf("upsert seo_state: %w", err)
	}
	return nil
}
