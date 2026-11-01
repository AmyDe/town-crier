package applications

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Sources reported by ReadPathState.Source.
const (
	ReadPathSourceDefault = "default"
	ReadPathSourceSet     = "set"
)

// ReadPathSwitches are the thin-table switches. ThinSync keeps
// application_locations in step with applications. ThinList moves the paged list
// reads onto it. ThinMap moves the map cluster reads onto it.
type ReadPathSwitches struct {
	ThinSync bool
	ThinList bool
	ThinMap  bool
}

// ReadPathControl is the stored read_path_control row.
type ReadPathControl struct {
	ReadPathSwitches
	Reason    string
	UpdatedAt time.Time
}

// ReadPathPatch names the switches to change. A nil field keeps its value.
type ReadPathPatch struct {
	ThinSync *bool
	ThinList *bool
	ThinMap  *bool
}

// ReadPathState is the effective control. Every switch is off, with Source
// ReadPathSourceDefault, until a row is stored.
type ReadPathState struct {
	ReadPathSwitches
	Source    string
	Reason    string
	UpdatedAt *time.Time
}

type readPathStore interface {
	Get(ctx context.Context) (ReadPathControl, bool, error)
	Set(ctx context.Context, c ReadPathControl) error
}

// ReadPathSwitch reads and writes the thin-table switches. It caches nothing:
// every State call reads the store.
type ReadPathSwitch struct {
	store readPathStore
}

// NewReadPathSwitch wires a ReadPathSwitch over store.
func NewReadPathSwitch(store readPathStore) *ReadPathSwitch {
	return &ReadPathSwitch{store: store}
}

// State returns the effective switches.
func (s *ReadPathSwitch) State(ctx context.Context) (ReadPathState, error) {
	c, found, err := s.store.Get(ctx)
	if err != nil {
		return ReadPathState{}, fmt.Errorf("read read_path_control: %w", err)
	}
	if !found {
		return ReadPathState{Source: ReadPathSourceDefault}, nil
	}
	at := c.UpdatedAt
	return ReadPathState{ReadPathSwitches: c.ReadPathSwitches, Source: ReadPathSourceSet, Reason: c.Reason, UpdatedAt: &at}, nil
}

// Set applies patch over the stored switches and returns the state read back
// from the store.
func (s *ReadPathSwitch) Set(ctx context.Context, patch ReadPathPatch, reason string, at time.Time) (ReadPathState, error) {
	cur, err := s.State(ctx)
	if err != nil {
		return ReadPathState{}, err
	}
	next := cur.ReadPathSwitches
	if patch.ThinSync != nil {
		next.ThinSync = *patch.ThinSync
	}
	if patch.ThinList != nil {
		next.ThinList = *patch.ThinList
	}
	if patch.ThinMap != nil {
		next.ThinMap = *patch.ThinMap
	}
	if err := s.store.Set(ctx, ReadPathControl{ReadPathSwitches: next, Reason: reason, UpdatedAt: at}); err != nil {
		return ReadPathState{}, fmt.Errorf("write read_path_control: %w", err)
	}
	return s.State(ctx)
}

// PostgresReadPathStore is the Postgres-backed single-row read_path_control table.
type PostgresReadPathStore struct {
	pool *pgxpool.Pool
}

// NewPostgresReadPathStore wires the store over pool.
func NewPostgresReadPathStore(pool *pgxpool.Pool) *PostgresReadPathStore {
	return &PostgresReadPathStore{pool: pool}
}

// Get returns the stored row, or false when none exists.
func (s *PostgresReadPathStore) Get(ctx context.Context) (ReadPathControl, bool, error) {
	var c ReadPathControl
	err := s.pool.QueryRow(ctx, "SELECT thin_sync, thin_list, thin_map, reason, updated_at FROM read_path_control WHERE id").
		Scan(&c.ThinSync, &c.ThinList, &c.ThinMap, &c.Reason, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ReadPathControl{}, false, nil
	}
	if err != nil {
		return ReadPathControl{}, false, fmt.Errorf("select read_path_control: %w", err)
	}
	return c, true, nil
}

// Set writes the single read_path_control row, replacing any previous value.
func (s *PostgresReadPathStore) Set(ctx context.Context, c ReadPathControl) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO read_path_control (id, thin_sync, thin_list, thin_map, reason, updated_at)
		VALUES (true, $1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET thin_sync = EXCLUDED.thin_sync, thin_list = EXCLUDED.thin_list,
			thin_map = EXCLUDED.thin_map, reason = EXCLUDED.reason, updated_at = EXCLUDED.updated_at`,
		c.ThinSync, c.ThinList, c.ThinMap, c.Reason, c.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert read_path_control: %w", err)
	}
	return nil
}

const readPathCacheTTL = 30 * time.Second

// CachedReadPath answers the read-path switches from an in-process copy of the
// control row, refreshed at most every 30 seconds. It is safe for concurrent use.
type CachedReadPath struct {
	store readPathStore
	now   func() time.Time

	mu       sync.Mutex
	switches ReadPathSwitches
	loadedAt time.Time
	loaded   bool
}

// NewCachedReadPath wires a CachedReadPath over store. now supplies the clock.
func NewCachedReadPath(store readPathStore, now func() time.Time) *CachedReadPath {
	return &CachedReadPath{store: store, now: now}
}

// Switches returns the cached switches. A missing row or a failed read means
// every switch is off, and that answer is cached like any other.
func (c *CachedReadPath) Switches(ctx context.Context) ReadPathSwitches {
	now := c.now()
	c.mu.Lock()
	if c.loaded && now.Sub(c.loadedAt) < readPathCacheTTL {
		sw := c.switches
		c.mu.Unlock()
		return sw
	}
	c.mu.Unlock()

	var sw ReadPathSwitches
	if row, found, err := c.store.Get(ctx); err == nil && found {
		sw = row.ReadPathSwitches
	}

	c.mu.Lock()
	c.switches, c.loadedAt, c.loaded = sw, now, true
	c.mu.Unlock()
	return sw
}

// ThinList reports whether the paged list reads use application_locations.
func (c *CachedReadPath) ThinList(ctx context.Context) bool {
	return c.Switches(ctx).ThinList
}

// ThinMap reports whether the map cluster reads use application_locations.
func (c *CachedReadPath) ThinMap(ctx context.Context) bool {
	return c.Switches(ctx).ThinMap
}
