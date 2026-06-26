package notifications

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// querier is the consumer-side slice of *pgxpool.Pool the store uses.
// Only Exec and Query are needed — all reads use Query + CollectRows,
// which keeps the interface fakeable for unit tests (no concrete pgx.Row).
// Both *pgxpool.Pool and pgx.Tx satisfy it structurally.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Store is the full exported method set that notifications consumers rely on.
// It serves two purposes: a compile-time parity check so *PostgresStore can
// never silently diverge from the Cosmos method set, and the consumer-side
// interface the API wiring accepts once the Postgres backend is selected.
//
// PurgeOlderThan is deliberately excluded: it has no Cosmos equivalent and is
// called only by the maintenance worker, not the common API paths.
type Store interface {
	Create(ctx context.Context, n DigestNotification) error
	GetLatestUnreadByApplications(ctx context.Context, userID string, applicationUIDs []string, lastReadAt time.Time) (map[string]LatestUnread, error)
	ByUserSince(ctx context.Context, userID string, since time.Time) ([]DigestNotification, error)
	AllByUser(ctx context.Context, userID string) ([]DigestNotification, error)
	UnsentEmailsByUser(ctx context.Context, userID string) ([]DigestNotification, error)
	UserIDsWithUnsentEmails(ctx context.Context) ([]string, error)
	MarkEmailSent(ctx context.Context, n DigestNotification) error
	GetByUserAndApplication(ctx context.Context, userID, applicationUID string, authorityID int, eventType EventType) (*DigestNotification, error)
	DeleteAllByUserID(ctx context.Context, userID string) error
}

// Compile-time parity: *PostgresStore must satisfy the full Store surface.
var _ Store = (*PostgresStore)(nil)

// PostgresStore reads and writes notifications in the Postgres `notifications`
// table (Cosmos → Postgres migration; memo 0010, epic #645).
//
// Key differences from the Cosmos split model (CosmosStore / DigestStore /
// DeleteStore across three types):
//   - Single struct over one table; the consumer-side interface union is
//     satisfied by one concrete type, so wiring is simpler.
//   - UserIDsWithUnsentEmails uses native SELECT DISTINCT — no cross-partition
//     client-side dedup needed (contrast tc-b7cm and the Cosmos gateway 400).
//   - The 90-day TTL is replaced by PurgeOlderThan; the INDEX on created_at
//     makes the DELETE efficient.
//   - GetLatestUnreadByApplications uses DISTINCT ON for the per-uid reduction,
//     replacing the Cosmos "first seen wins on newest-first ordered results".
type PostgresStore struct {
	db querier
}

// NewPostgresStore returns a store over the given pgx pool (or any querier).
func NewPostgresStore(db querier) *PostgresStore {
	return &PostgresStore{db: db}
}

// notifColumns is the full read projection for DigestNotification rows.
// Its order MUST match scanDigestRow.
const notifColumns = "id, user_id, application_uid, application_name, watch_zone_id, " +
	"application_address, application_description, application_type, " +
	"authority_id, decision, event_type, sources, push_sent, email_sent, created_at"

// scanDigestRow hydrates one row into a full DigestNotification. Used by
// ByUserSince, AllByUser, UnsentEmailsByUser, and GetByUserAndApplication.
func scanDigestRow(row pgx.CollectableRow) (DigestNotification, error) {
	var (
		id, userID, applicationUID, applicationName string
		watchZoneID                                 *string
		applicationAddress, applicationDescription  string
		applicationType                             *string
		authorityID                                 int
		decision                                    *string
		eventType, sources                          string
		pushSent, emailSent                         bool
		createdAt                                   time.Time
	)
	if err := row.Scan(
		&id, &userID, &applicationUID, &applicationName, &watchZoneID,
		&applicationAddress, &applicationDescription, &applicationType,
		&authorityID, &decision, &eventType, &sources, &pushSent, &emailSent, &createdAt,
	); err != nil {
		return DigestNotification{}, err
	}
	et := EventNewApplication
	if eventType != "" {
		et = EventType(eventType)
	}
	return DigestNotification{
		ID:                     id,
		UserID:                 userID,
		ApplicationUID:         applicationUID,
		ApplicationName:        applicationName,
		WatchZoneID:            watchZoneID,
		ApplicationAddress:     applicationAddress,
		ApplicationDescription: applicationDescription,
		ApplicationType:        applicationType,
		AuthorityID:            authorityID,
		Decision:               decision,
		EventType:              et,
		Sources:                sources,
		PushSent:               pushSent,
		EmailSent:              emailSent,
		CreatedAt:              createdAt,
	}, nil
}

// scanLatestUnreadRow hydrates the minimal 4-column projection used by
// GetLatestUnreadByApplications (application_uid, decision, event_type,
// created_at). Coalesces an empty event_type to NewApplication so legacy
// rows (if any exist post-backfill) remain safe.
func scanLatestUnreadRow(row pgx.CollectableRow) (LatestUnread, error) {
	var (
		applicationUID string
		decision       *string
		eventType      string
		createdAt      time.Time
	)
	if err := row.Scan(&applicationUID, &decision, &eventType, &createdAt); err != nil {
		return LatestUnread{}, err
	}
	et := EventNewApplication
	if eventType != "" {
		et = EventType(eventType)
	}
	return LatestUnread{
		ApplicationUID: applicationUID,
		EventType:      et,
		Decision:       decision,
		CreatedAt:      createdAt,
	}, nil
}

const pgCreateQuery = `
INSERT INTO notifications (
    id, user_id, application_uid, application_name, watch_zone_id,
    application_address, application_description, application_type,
    authority_id, decision, event_type, sources, push_sent, email_sent, created_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
)
ON CONFLICT (id) DO UPDATE SET
    user_id                 = EXCLUDED.user_id,
    application_uid         = EXCLUDED.application_uid,
    application_name        = EXCLUDED.application_name,
    watch_zone_id           = EXCLUDED.watch_zone_id,
    application_address     = EXCLUDED.application_address,
    application_description = EXCLUDED.application_description,
    application_type        = EXCLUDED.application_type,
    authority_id            = EXCLUDED.authority_id,
    decision                = EXCLUDED.decision,
    event_type              = EXCLUDED.event_type,
    sources                 = EXCLUDED.sources,
    push_sent               = EXCLUDED.push_sent,
    email_sent              = EXCLUDED.email_sent,
    created_at              = EXCLUDED.created_at`

// Create writes a dispatched notification. Idempotent on the notification id
// (ON CONFLICT (id) DO UPDATE), mirroring the Cosmos UpsertItem-by-document-id
// contract: re-creating the same id overwrites in place.
func (s *PostgresStore) Create(ctx context.Context, n DigestNotification) error {
	_, err := s.db.Exec(ctx, pgCreateQuery,
		n.ID, n.UserID, n.ApplicationUID, n.ApplicationName, n.WatchZoneID,
		n.ApplicationAddress, n.ApplicationDescription, n.ApplicationType,
		n.AuthorityID, n.Decision, string(n.EventType), n.Sources,
		n.PushSent, n.EmailSent, n.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create notification %q: %w", n.ID, err)
	}
	return nil
}

// pgLatestUnreadQuery uses DISTINCT ON (application_uid) to return the newest
// notification per uid in one pass — the Postgres equivalent of the Cosmos
// "first row seen on a newest-first result set" reduction. The 4-column
// projection avoids reading full document bodies for a display-only badge.
const pgLatestUnreadQuery = "SELECT DISTINCT ON (application_uid) " +
	"application_uid, decision, event_type, created_at " +
	"FROM notifications " +
	"WHERE user_id = $1 AND application_uid = ANY($2) AND created_at > $3 " +
	"ORDER BY application_uid, created_at DESC"

// GetLatestUnreadByApplications returns, for each application uid that has at
// least one notification created strictly after lastReadAt, the latest such
// notification — in a single round trip instead of N+1 per-uid queries.
// An empty uid set returns an empty map without issuing a query, matching the
// Cosmos early-return guard.
func (s *PostgresStore) GetLatestUnreadByApplications(ctx context.Context, userID string, applicationUIDs []string, lastReadAt time.Time) (map[string]LatestUnread, error) {
	if len(applicationUIDs) == 0 {
		return map[string]LatestUnread{}, nil
	}
	rows, err := s.db.Query(ctx, pgLatestUnreadQuery, userID, applicationUIDs, lastReadAt)
	if err != nil {
		return nil, fmt.Errorf("query latest unread for %q: %w", userID, err)
	}
	items, err := pgx.CollectRows(rows, scanLatestUnreadRow)
	if err != nil {
		return nil, fmt.Errorf("scan latest unread for %q: %w", userID, err)
	}
	latest := make(map[string]LatestUnread, len(items))
	for _, lu := range items {
		latest[lu.ApplicationUID] = lu
	}
	return latest, nil
}

const pgByUserSinceQuery = "SELECT " + notifColumns +
	" FROM notifications WHERE user_id = $1 AND created_at >= $2 ORDER BY created_at DESC"

// ByUserSince returns the user's notifications created at or after since,
// newest first — the weekly-digest window.
func (s *PostgresStore) ByUserSince(ctx context.Context, userID string, since time.Time) ([]DigestNotification, error) {
	rows, err := s.db.Query(ctx, pgByUserSinceQuery, userID, since)
	if err != nil {
		return nil, fmt.Errorf("query notifications since for %q: %w", userID, err)
	}
	items, err := pgx.CollectRows(rows, scanDigestRow)
	if err != nil {
		return nil, fmt.Errorf("scan notifications since for %q: %w", userID, err)
	}
	if items == nil {
		return []DigestNotification{}, nil
	}
	return items, nil
}

const pgAllByUserQuery = "SELECT " + notifColumns +
	" FROM notifications WHERE user_id = $1 ORDER BY created_at ASC"

// AllByUser returns every notification for the user, oldest first, for the
// GDPR data export (GET /v1/me/data). No time window — the export covers the
// whole notification history (90 days bounded by PurgeOlderThan, analogous to
// the Cosmos 90-day TTL).
func (s *PostgresStore) AllByUser(ctx context.Context, userID string) ([]DigestNotification, error) {
	rows, err := s.db.Query(ctx, pgAllByUserQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("query all notifications for %q: %w", userID, err)
	}
	items, err := pgx.CollectRows(rows, scanDigestRow)
	if err != nil {
		return nil, fmt.Errorf("scan all notifications for %q: %w", userID, err)
	}
	if items == nil {
		return []DigestNotification{}, nil
	}
	return items, nil
}

const pgUnsentEmailsQuery = "SELECT " + notifColumns +
	" FROM notifications WHERE user_id = $1 AND NOT email_sent ORDER BY created_at ASC"

// UnsentEmailsByUser returns the user's notifications awaiting an email
// (email_sent = false), oldest first — the hourly-digest pipeline's per-user
// read. Replaces the Cosmos OR NOT IS_DEFINED(emailSent) guard: every PG row
// has email_sent NOT NULL DEFAULT false, so legacy-row handling is unnecessary.
func (s *PostgresStore) UnsentEmailsByUser(ctx context.Context, userID string) ([]DigestNotification, error) {
	rows, err := s.db.Query(ctx, pgUnsentEmailsQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("query unsent emails for %q: %w", userID, err)
	}
	items, err := pgx.CollectRows(rows, scanDigestRow)
	if err != nil {
		return nil, fmt.Errorf("scan unsent emails for %q: %w", userID, err)
	}
	if items == nil {
		return []DigestNotification{}, nil
	}
	return items, nil
}

const pgUserIDsWithUnsentEmailsQuery = "SELECT DISTINCT user_id FROM notifications WHERE NOT email_sent"

// UserIDsWithUnsentEmails returns every user id with at least one unsent-email
// notification — the hourly cycle's candidate set. Uses native DISTINCT instead
// of the Cosmos cross-partition client-side dedup (the gateway 400 on
// cross-partition DISTINCT; tc-b7cm does not apply here). A defensive client-
// side dedup is retained to preserve the contract regardless of the SQL.
func (s *PostgresStore) UserIDsWithUnsentEmails(ctx context.Context) ([]string, error) {
	rows, err := s.db.Query(ctx, pgUserIDsWithUnsentEmailsQuery)
	if err != nil {
		return nil, fmt.Errorf("query users with unsent emails: %w", err)
	}
	raw, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("scan users with unsent emails: %w", err)
	}
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, id := range raw {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

const pgMarkEmailSentQuery = "UPDATE notifications SET email_sent = true WHERE id = $1"

// MarkEmailSent flips email_sent on the notification so it is excluded from
// the next hourly cycle, matching the Cosmos UpsertItem path that re-writes
// the full document with EmailSent = true.
func (s *PostgresStore) MarkEmailSent(ctx context.Context, n DigestNotification) error {
	if _, err := s.db.Exec(ctx, pgMarkEmailSentQuery, n.ID); err != nil {
		return fmt.Errorf("mark email sent for notification %q: %w", n.ID, err)
	}
	return nil
}

const pgGetByUserAndApplicationQuery = "SELECT " + notifColumns +
	" FROM notifications" +
	" WHERE user_id = $1 AND application_uid = $2 AND authority_id = $3 AND event_type = $4 LIMIT 1"

// GetByUserAndApplication returns the user's existing notification for the
// (applicationUID, authorityID, eventType) tuple, or nil when none exists —
// the "not yet notified" dedup signal the poll fan-out branches on. The UNIQUE
// constraint on (user_id, application_uid, authority_id, event_type) ensures at
// most one row matches; LIMIT 1 makes the planner's intent explicit.
func (s *PostgresStore) GetByUserAndApplication(ctx context.Context, userID, applicationUID string, authorityID int, eventType EventType) (*DigestNotification, error) {
	rows, err := s.db.Query(ctx, pgGetByUserAndApplicationQuery,
		userID, applicationUID, authorityID, string(eventType))
	if err != nil {
		return nil, fmt.Errorf("query existing notification for %q: %w", userID, err)
	}
	items, err := pgx.CollectRows(rows, scanDigestRow)
	if err != nil {
		return nil, fmt.Errorf("scan existing notification for %q: %w", userID, err)
	}
	if len(items) == 0 {
		return nil, nil //nolint:nilnil // absent notification is the "not yet notified" signal, not an error
	}
	return &items[0], nil
}

const pgDeleteAllByUserIDQuery = "DELETE FROM notifications WHERE user_id = $1"

// DeleteAllByUserID removes every notification for the user — the GDPR
// Art. 17 erasure cascade (erasure.ChildDeleter). Deleting zero rows is not
// an error.
func (s *PostgresStore) DeleteAllByUserID(ctx context.Context, userID string) error {
	if _, err := s.db.Exec(ctx, pgDeleteAllByUserIDQuery, userID); err != nil {
		return fmt.Errorf("delete all notifications for %q: %w", userID, err)
	}
	return nil
}

const pgPurgeOlderThanQuery = "DELETE FROM notifications WHERE created_at < $1"

// PurgeOlderThan deletes every notification created before cutoff and returns
// the number of rows deleted. It is the Postgres replacement for the Cosmos
// 90-day TTL: a maintenance worker (later slice) calls it on a schedule.
// The INDEX on created_at makes the full-table sweep efficient.
func (s *PostgresStore) PurgeOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := s.db.Exec(ctx, pgPurgeOlderThanQuery, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purge notifications older than %v: %w", cutoff, err)
	}
	return tag.RowsAffected(), nil
}
