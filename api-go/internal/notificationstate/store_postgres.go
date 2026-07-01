package notificationstate

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// querier is the consumer-side slice of *pgxpool.Pool the store uses.
// Only Exec and Query are needed — all reads use Query + CollectRows,
// keeping the interface fakeable for unit tests. Both *pgxpool.Pool and
// pgx.Tx satisfy it structurally.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Store is the full exported method set the notificationstate consumers rely
// on. It serves two purposes: compile-time parity so *PostgresStore can never
// silently diverge from the method set callers need, and the consumer-side
// interface the API wiring accepts once the Postgres backend is selected.
type Store interface {
	Get(ctx context.Context, userID string) (*State, error)
	Save(ctx context.Context, st State) error
	UnreadCount(ctx context.Context, userID string) (int, error)
	MarkAllRead(ctx context.Context, userID string, now time.Time) (int64, error)
	MarkApplicationsRead(ctx context.Context, userID string, refs []string, authorityIDs []int, now time.Time) (int64, error)
	// MarkReadUpTo is a TEMPORARY backward-compat shim for the retired
	// scroll-to-clear watermark (see the method doc and pgMarkReadUpToQuery).
	// REMOVE per bead tc-v5w8 once the per-app read-state iOS build is live.
	MarkReadUpTo(ctx context.Context, userID string, asOf, now time.Time) (int64, error)
	DeleteByUserID(ctx context.Context, userID string) error
}

// Compile-time parity: *PostgresStore must satisfy the full Store surface.
var _ Store = (*PostgresStore)(nil)

// PostgresStore owns the per-user notification read state: the change-token row
// in `notification_state` and the read-state mutations over the `notifications`
// table (read_at). Both tables live in the same pool, so a mutation and its
// version bump run atomically in a single data-modifying CTE statement.
//
//   - Unread is read_at IS NULL (ADR 0035); UnreadCount is a SELECT count(*)
//     over the partial index idx_notifications_unread.
//   - MarkApplicationsRead clears the caller's unread rows for a set of
//     (application_name, authority_id) pairs — it matches application_name (=
//     a.Name, the PlanIt case reference the clients and push payload carry), NOT
//     application_uid (#733). The composite is load-bearing: a.Name is unique
//     within a council but collides across councils, so authority_id disambiguates.
//     It bumps the version token only when it cleared a row.
//   - MarkAllRead clears every unread row for the user and always bumps the
//     version token (upserting the state row when absent).
type PostgresStore struct {
	db querier
}

// NewPostgresStore returns a store over the given pgx pool (or any querier).
func NewPostgresStore(db querier) *PostgresStore {
	return &PostgresStore{db: db}
}

// scanStateRow hydrates one row from notification_state.
// Column order: user_id, last_read_at, version — MUST match all SELECT queries.
func scanStateRow(row pgx.CollectableRow) (State, error) {
	var (
		userID     string
		lastReadAt time.Time
		version    int
	)
	if err := row.Scan(&userID, &lastReadAt, &version); err != nil {
		return State{}, err
	}
	return State{
		UserID:     userID,
		LastReadAt: lastReadAt,
		Version:    version,
	}, nil
}

const pgGetStateQuery = "SELECT user_id, last_read_at, version FROM notification_state WHERE user_id = $1"

// Get point-reads the user's watermark. A missing row returns (nil, nil) —
// the first-touch signal the handlers branch on, identical to the Cosmos
// 404-on-ReadItem behaviour.
func (s *PostgresStore) Get(ctx context.Context, userID string) (*State, error) {
	rows, err := s.db.Query(ctx, pgGetStateQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("read notification state %q: %w", userID, err)
	}
	items, err := pgx.CollectRows(rows, scanStateRow)
	if err != nil {
		return nil, fmt.Errorf("scan notification state %q: %w", userID, err)
	}
	if len(items) == 0 {
		return nil, nil //nolint:nilnil // absent watermark is the first-touch signal, not an error
	}
	st := items[0]
	return &st, nil
}

const pgSaveStateQuery = `
INSERT INTO notification_state (user_id, last_read_at, version)
VALUES ($1, $2, $3)
ON CONFLICT (user_id) DO UPDATE SET
    last_read_at = EXCLUDED.last_read_at,
    version      = EXCLUDED.version`

// Save upserts the watermark, matching the Cosmos UpsertItem-by-userId
// contract.
func (s *PostgresStore) Save(ctx context.Context, st State) error {
	if _, err := s.db.Exec(ctx, pgSaveStateQuery, st.UserID, st.LastReadAt, st.Version); err != nil {
		return fmt.Errorf("upsert notification state %q: %w", st.UserID, err)
	}
	return nil
}

// pgUnreadCountQuery counts the user's unread notifications (read_at IS NULL),
// served by the partial index idx_notifications_unread (ADR 0035).
const pgUnreadCountQuery = "SELECT count(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL"

// UnreadCount counts the user's unread notifications (read_at IS NULL).
func (s *PostgresStore) UnreadCount(ctx context.Context, userID string) (int, error) {
	rows, err := s.db.Query(ctx, pgUnreadCountQuery, userID)
	if err != nil {
		return 0, fmt.Errorf("count unread for %q: %w", userID, err)
	}
	counts, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return 0, fmt.Errorf("scan unread count for %q: %w", userID, err)
	}
	if len(counts) == 0 {
		return 0, nil
	}
	return int(counts[0]), nil
}

// pgMarkAllReadQuery clears every unread notification for the user (read_at set
// to now) and unconditionally bumps the version change token, upserting the
// state row when the user has none. Both tables mutate atomically in one
// data-modifying CTE statement; the top-level SELECT returns the cleared count.
const pgMarkAllReadQuery = `
WITH cleared AS (
    UPDATE notifications
    SET read_at = $2
    WHERE user_id = $1 AND read_at IS NULL
    RETURNING 1
), bumped AS (
    INSERT INTO notification_state (user_id, last_read_at, version)
    VALUES ($1, $2, 1)
    ON CONFLICT (user_id) DO UPDATE SET
        last_read_at = EXCLUDED.last_read_at,
        version      = notification_state.version + 1
    RETURNING 1
)
SELECT count(*) FROM cleared`

// MarkAllRead clears all of the user's unread notifications and bumps the
// version change token (upserting the state row if absent). It returns the
// number of notifications cleared. The version bump is unconditional so
// BadgeSync still observes a change even when nothing was unread.
func (s *PostgresStore) MarkAllRead(ctx context.Context, userID string, now time.Time) (int64, error) {
	rows, err := s.db.Query(ctx, pgMarkAllReadQuery, userID, now)
	if err != nil {
		return 0, fmt.Errorf("mark all read for %q: %w", userID, err)
	}
	counts, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return 0, fmt.Errorf("scan mark all read count for %q: %w", userID, err)
	}
	if len(counts) == 0 {
		return 0, nil
	}
	return counts[0], nil
}

// pgMarkApplicationsReadQuery clears the caller's unread notifications for a set
// of (application_name, authority_id) pairs supplied as two parallel arrays, and
// bumps the version change token only when it actually cleared a row (upserting
// the state row when absent).
//
// It matches application_name, NOT application_uid — this is the #733 fix and is
// load-bearing. The `ref` values in $3 are PlanIt CASE REFERENCES (= a.Name, e.g.
// "24/0001"), which is what every caller carries: the push payload sets
// applicationRef = n.ApplicationName (notifydispatch/payload.go), iOS sends id.name,
// and web sends summary.name. The `application_uid` column instead holds a.UID (e.g.
// "24/0001/FUL") and no client ever sends it, so the previous application_uid match
// silently cleared zero rows in production. a.Name is unique within a council but
// collides across councils, so authority_id disambiguates — the composite pair is
// therefore load-bearing. Empty arrays match nothing (a 204 no-op), never "all".
// Both tables mutate atomically in one CTE statement; the top-level SELECT returns
// the cleared count.
const pgMarkApplicationsReadQuery = `
WITH cleared AS (
    UPDATE notifications n
    SET read_at = $2
    FROM unnest($3::text[], $4::int[]) AS t(ref, aid)
    WHERE n.user_id = $1
      AND n.application_name = t.ref
      AND n.authority_id = t.aid
      AND n.read_at IS NULL
    RETURNING 1
), bumped AS (
    INSERT INTO notification_state (user_id, last_read_at, version)
    SELECT $1, $2, 1
    WHERE EXISTS (SELECT 1 FROM cleared)
    ON CONFLICT (user_id) DO UPDATE SET version = notification_state.version + 1
    RETURNING 1
)
SELECT count(*) FROM cleared`

// MarkApplicationsRead clears the caller's unread notifications for the given
// (ref, authorityID) pairs and bumps the version change token when it cleared at
// least one row (leaving the token untouched on a zero-row no-op, so mark-read
// stays idempotent). refs and authorityIDs are parallel: the i-th pair is
// (refs[i], authorityIDs[i]). Each ref is a PlanIt case reference (= a.Name),
// matched against application_name — see pgMarkApplicationsReadQuery for why it is
// NOT application_uid. It returns the number of notifications cleared.
func (s *PostgresStore) MarkApplicationsRead(ctx context.Context, userID string, refs []string, authorityIDs []int, now time.Time) (int64, error) {
	rows, err := s.db.Query(ctx, pgMarkApplicationsReadQuery, userID, now, refs, authorityIDs)
	if err != nil {
		return 0, fmt.Errorf("mark applications read for %q: %w", userID, err)
	}
	counts, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return 0, fmt.Errorf("scan mark applications read count for %q: %w", userID, err)
	}
	if len(counts) == 0 {
		return 0, nil
	}
	return counts[0], nil
}

// pgMarkReadUpToQuery is the read_at-model translation of the retired
// scroll-to-clear watermark advance. It clears every unread notification created
// at or before asOf (the read_at equivalent of moving a watermark to asOf) and
// bumps the version change token only when it actually cleared a row (upserting
// the state row when absent) — the same atomic CTE style as
// pgMarkApplicationsReadQuery. Both tables mutate in one data-modifying
// statement; the top-level SELECT returns the cleared count.
//
// TEMPORARY BACKWARD-COMPAT SHIM (tc-ekii). ADR 0035 (#733) removed the watermark
// advance in favour of per-application read_at, so new iOS/web clients do NOT call
// advance (they use POST /v1/me/applications/mark-read). This query exists only to
// keep the App Store iOS builds that predate that change (still live + one in Apple
// review) clearing their push badge on tap during the review window. REMOVE per bead
// tc-v5w8 once the new iOS build is live; advance 404-ing again is the #733/ADR-0035
// end-state.
//
// $1 userID, $2 asOf, $3 now.
const pgMarkReadUpToQuery = `
WITH cleared AS (
    UPDATE notifications
    SET read_at = $3
    WHERE user_id = $1 AND created_at <= $2 AND read_at IS NULL
    RETURNING 1
), bumped AS (
    INSERT INTO notification_state (user_id, last_read_at, version)
    SELECT $1, $3, 1 WHERE EXISTS (SELECT 1 FROM cleared)
    ON CONFLICT (user_id) DO UPDATE SET version = notification_state.version + 1
    RETURNING 1
)
SELECT count(*) FROM cleared`

// MarkReadUpTo clears every unread notification for the user created at or before
// asOf and bumps the version change token when it cleared at least one row
// (leaving the token untouched on a zero-row no-op, so a repeat advance is
// idempotent). It returns the number of notifications cleared. This is the
// read_at-model equivalent of the retired watermark advance-to-asOf.
//
// TEMPORARY BACKWARD-COMPAT SHIM (tc-ekii) — see pgMarkReadUpToQuery. Called only
// by the re-added POST /v1/me/notification-state/advance route, which exists purely
// to keep pre-per-app-read-state iOS clients (App Store live + Apple review) clearing
// their badge on push-tap. New iOS/web clients use MarkApplicationsRead instead.
// REMOVE per bead tc-v5w8 once the new iOS build is live.
func (s *PostgresStore) MarkReadUpTo(ctx context.Context, userID string, asOf, now time.Time) (int64, error) {
	rows, err := s.db.Query(ctx, pgMarkReadUpToQuery, userID, asOf, now)
	if err != nil {
		return 0, fmt.Errorf("mark read up to for %q: %w", userID, err)
	}
	counts, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return 0, fmt.Errorf("scan mark read up to count for %q: %w", userID, err)
	}
	if len(counts) == 0 {
		return 0, nil
	}
	return counts[0], nil
}

const pgDeleteStateQuery = "DELETE FROM notification_state WHERE user_id = $1"

// DeleteByUserID removes the user's watermark — the GDPR Art. 17 erasure
// cascade (bridged to erasure.ChildDeleter by erasure.NotificationStateChild).
// A missing row (no watermark yet) is not an error, matching the Cosmos
// 404-tolerant DeleteItem behaviour.
func (s *PostgresStore) DeleteByUserID(ctx context.Context, userID string) error {
	if _, err := s.db.Exec(ctx, pgDeleteStateQuery, userID); err != nil {
		return fmt.Errorf("delete notification state %q: %w", userID, err)
	}
	return nil
}
