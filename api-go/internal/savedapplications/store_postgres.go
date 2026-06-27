package savedapplications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
)

// querier is the consumer-side slice of *pgxpool.Pool the store uses:
// parameterised exec/query/query-row. Both *pgxpool.Pool and pgx.Tx satisfy it
// structurally, so the store is testable without a real connection.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Store is the full saved-application method set *PostgresStore satisfies and the
// exported consumer-side interface the handlers and wiring depend on.
type Store interface {
	Save(ctx context.Context, sa SavedApplication) error
	Exists(ctx context.Context, userID, applicationUID string) (bool, error)
	Delete(ctx context.Context, userID, applicationUID string) error
	GetByUserID(ctx context.Context, userID string) ([]SavedApplication, error)
	UserIDsForApplication(ctx context.Context, applicationUID string, authorityID int) ([]string, error)
	DeleteAllByUserID(ctx context.Context, userID string) error
}

// Compile-time check: the store satisfies the consumer-side Store interface.
var _ Store = (*PostgresStore)(nil)

// PostgresStore reads and writes saved applications in the Postgres
// `saved_applications` table (Cosmos → Postgres migration; memo 0010, epic #645).
//
// Snapshot: the embedded applications.SnapshotDocument is stored as jsonb, so
// every field the export/refresh path needs survives the round-trip without loss.
// A nil snapshot stores NULL.
//
// UserIDsForApplication scopes on both application_uid AND authority_id, matching
// the Cosmos impl exactly (PlanIt uids collide across councils — tc-th98 / GH#384,
// so a uid-only match would falsely fan out a decision to bookmark holders in
// another authority). The (application_uid, authority_id) composite index makes
// this query index-served on the hot poll path.
type PostgresStore struct {
	db querier
}

// NewPostgresStore returns a store backed by the given pgx pool or querier.
func NewPostgresStore(db querier) *PostgresStore {
	return &PostgresStore{db: db}
}

const pgSaveSavedAppQuery = `
INSERT INTO saved_applications (user_id, application_uid, authority_id, saved_at, snapshot)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id, application_uid) DO UPDATE SET
    authority_id = EXCLUDED.authority_id,
    saved_at     = EXCLUDED.saved_at,
    snapshot     = EXCLUDED.snapshot`

// Save upserts the saved application keyed on (user_id, application_uid). The
// full embedded snapshot is serialised to jsonb so the list endpoint and snapshot
// refresher need no extra hydration query.
func (s *PostgresStore) Save(ctx context.Context, sa SavedApplication) error {
	var snapshotJSON []byte
	if sa.Application != nil {
		doc := applications.NewSnapshotDocument(*sa.Application)
		var err error
		snapshotJSON, err = json.Marshal(doc)
		if err != nil {
			return fmt.Errorf("encode snapshot for %q: %w", sa.ApplicationUID, err)
		}
	}
	if _, err := s.db.Exec(ctx, pgSaveSavedAppQuery,
		sa.UserID, sa.ApplicationUID, sa.AuthorityID, sa.SavedAt, snapshotJSON); err != nil {
		return fmt.Errorf("upsert saved application %q: %w", sa.ApplicationUID, err)
	}
	return nil
}

const pgExistsSavedAppQuery = "SELECT 1 FROM saved_applications WHERE user_id = $1 AND application_uid = $2"

// Exists reports whether the user has saved the application with the given uid.
func (s *PostgresStore) Exists(ctx context.Context, userID, applicationUID string) (bool, error) {
	var dummy int
	err := s.db.QueryRow(ctx, pgExistsSavedAppQuery, userID, applicationUID).Scan(&dummy)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check saved application %q: %w", applicationUID, err)
	}
	return true, nil
}

const pgDeleteSavedAppQuery = "DELETE FROM saved_applications WHERE user_id = $1 AND application_uid = $2"

// Delete removes the saved application. A missing row is not an error: the DELETE
// endpoint is idempotent (always returns 204).
func (s *PostgresStore) Delete(ctx context.Context, userID, applicationUID string) error {
	if _, err := s.db.Exec(ctx, pgDeleteSavedAppQuery, userID, applicationUID); err != nil {
		return fmt.Errorf("delete saved application %q: %w", applicationUID, err)
	}
	return nil
}

const pgGetByUserIDQuery = "SELECT user_id, application_uid, authority_id, saved_at, snapshot " +
	"FROM saved_applications WHERE user_id = $1 ORDER BY saved_at"

// GetByUserID returns the user's saved applications, ordered by saved_at.
func (s *PostgresStore) GetByUserID(ctx context.Context, userID string) ([]SavedApplication, error) {
	rows, err := s.db.Query(ctx, pgGetByUserIDQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("query saved applications for %q: %w", userID, err)
	}
	defer rows.Close()

	var saved []SavedApplication
	for rows.Next() {
		sa, err := scanSavedApp(rows)
		if err != nil {
			return nil, fmt.Errorf("scan saved application for %q: %w", userID, err)
		}
		saved = append(saved, sa)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("saved application rows for %q: %w", userID, err)
	}
	return saved, nil
}

// scanSavedApp hydrates one SavedApplication from a pgx.Row (or pgx.Rows, which
// satisfies pgx.Row structurally). The jsonb snapshot column is unmarshalled back
// through applications.SnapshotDocument.ToDomain so all fields round-trip cleanly.
func scanSavedApp(row pgx.Row) (SavedApplication, error) {
	var (
		userID         string
		applicationUID string
		authorityID    int
		savedAt        time.Time
		snapshotJSON   []byte
	)
	if err := row.Scan(&userID, &applicationUID, &authorityID, &savedAt, &snapshotJSON); err != nil {
		return SavedApplication{}, err
	}
	sa := SavedApplication{
		UserID:         userID,
		ApplicationUID: applicationUID,
		AuthorityID:    authorityID,
		SavedAt:        savedAt,
	}
	if len(snapshotJSON) > 0 {
		var doc applications.SnapshotDocument
		if err := json.Unmarshal(snapshotJSON, &doc); err != nil {
			return SavedApplication{}, fmt.Errorf("decode snapshot for %q: %w", applicationUID, err)
		}
		app := doc.ToDomain()
		sa.Application = &app
	}
	return sa, nil
}

// pgUserIDsForApplicationQuery matches the Cosmos impl's authority predicate
// exactly: PlanIt uids collide across councils, so scoping on authority_id is
// load-bearing, not an optional optimisation.
const pgUserIDsForApplicationQuery = "SELECT DISTINCT user_id FROM saved_applications " +
	"WHERE application_uid = $1 AND authority_id = $2"

// UserIDsForApplication returns every distinct user id that has saved the given
// (applicationUID, authorityID). It backs the poll-path decision-event fan-out to
// bookmark holders. The query is index-served via the composite
// (application_uid, authority_id) index on the hot poll path.
func (s *PostgresStore) UserIDsForApplication(ctx context.Context, applicationUID string, authorityID int) ([]string, error) {
	rows, err := s.db.Query(ctx, pgUserIDsForApplicationQuery, applicationUID, authorityID)
	if err != nil {
		return nil, fmt.Errorf("query user ids for saved application %q: %w", applicationUID, err)
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, fmt.Errorf("scan user id for saved application %q: %w", applicationUID, err)
		}
		userIDs = append(userIDs, uid)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("user id rows for saved application %q: %w", applicationUID, err)
	}
	return userIDs, nil
}

const pgDeleteAllSavedAppsQuery = "DELETE FROM saved_applications WHERE user_id = $1"

// DeleteAllByUserID removes every saved application for the user. Used by the
// GDPR erasure cascade (dormant cleanup and DELETE /v1/me).
func (s *PostgresStore) DeleteAllByUserID(ctx context.Context, userID string) error {
	if _, err := s.db.Exec(ctx, pgDeleteAllSavedAppsQuery, userID); err != nil {
		return fmt.Errorf("delete all saved applications for %q: %w", userID, err)
	}
	return nil
}
