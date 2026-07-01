package profiles

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// PostgresAdminStore serves the admin-surface's cross-user profile operations:
// find-by-email, find-by-original-transaction-id, digest-day candidates,
// dormant-account scan, lapsed-paid sweep, save (grant write-back), and the
// paged user list. It is the Postgres counterpart of *AdminStore (Cosmos) and
// shares the same AdminProfileStore interface.
//
// The querier type is defined in store_postgres.go (same package). Both
// *pgxpool.Pool and pgx.Tx satisfy it structurally.
type PostgresAdminStore struct {
	db querier
}

// NewPostgresAdminStore returns an admin store over the given pgx pool (or any
// querier).
func NewPostgresAdminStore(db querier) *PostgresAdminStore {
	return &PostgresAdminStore{db: db}
}

// collectUsers drains pgx.Rows into a []*UserProfile slice, scanning each row
// with scanUserRow. It closes the rows on return and propagates Rows.Err().
func collectUsers(rows pgx.Rows) ([]*UserProfile, error) {
	defer rows.Close()
	var profiles []*UserProfile
	for rows.Next() {
		p, _, err := scanUserRow(rows)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return profiles, nil
}

const pgAdminSelectCols = "SELECT " + userSelectCols + " FROM users "

// GetByEmail returns the first profile whose email matches exactly, or
// ErrNotFound. Mirrors AdminStore.GetByEmail.
func (s *PostgresAdminStore) GetByEmail(ctx context.Context, email string) (*UserProfile, error) {
	rows, err := s.db.Query(ctx, pgAdminSelectCols+"WHERE email = $1 LIMIT 1", email)
	if err != nil {
		return nil, fmt.Errorf("query profile by email: %w", err)
	}
	profiles, err := collectUsers(rows)
	if err != nil {
		return nil, fmt.Errorf("decode profile by email: %w", err)
	}
	if len(profiles) == 0 {
		return nil, ErrNotFound
	}
	return profiles[0], nil
}

// GetByOriginalTransactionID returns the profile whose stored Apple original
// transaction id matches, or ErrNotFound. The App Store Server Notification
// webhook locates the subscriber by this cross-user lookup.
func (s *PostgresAdminStore) GetByOriginalTransactionID(ctx context.Context, originalTransactionID string) (*UserProfile, error) {
	rows, err := s.db.Query(ctx, pgAdminSelectCols+"WHERE original_transaction_id = $1 LIMIT 1", originalTransactionID)
	if err != nil {
		return nil, fmt.Errorf("query profile by original transaction id: %w", err)
	}
	profiles, err := collectUsers(rows)
	if err != nil {
		return nil, fmt.Errorf("decode profile by original transaction id: %w", err)
	}
	if len(profiles) == 0 {
		return nil, ErrNotFound
	}
	return profiles[0], nil
}

// ByDigestDay returns every profile whose configured digest day matches day.
// The digest day is stored as the int weekday value (Sunday=0 … Saturday=6).
func (s *PostgresAdminStore) ByDigestDay(ctx context.Context, day time.Weekday) ([]*UserProfile, error) {
	rows, err := s.db.Query(ctx, pgAdminSelectCols+"WHERE digest_day = $1", int(day))
	if err != nil {
		return nil, fmt.Errorf("query profiles by digest day: %w", err)
	}
	profiles, err := collectUsers(rows)
	if err != nil {
		return nil, fmt.Errorf("decode profiles by digest day: %w", err)
	}
	return profiles, nil
}

// Dormant returns every profile last active strictly before cutoff. The filter
// runs server-side on last_active_at_epoch (the numeric Unix-millisecond
// mirror) which sorts unambiguously — the Go-side LastActiveAt.Before(cutoff)
// check below is the final authority and remains the same as the Cosmos store
// (it guards against any residual clock drift or NULL epoch). Mirrors
// AdminStore.Dormant exactly, including the server-side epoch pre-filter and
// the Go-side strictly-before gate.
func (s *PostgresAdminStore) Dormant(ctx context.Context, cutoff time.Time) ([]*UserProfile, error) {
	rows, err := s.db.Query(ctx,
		pgAdminSelectCols+"WHERE last_active_at_epoch < $1",
		cutoff.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("query dormant profiles: %w", err)
	}
	candidates, err := collectUsers(rows)
	if err != nil {
		return nil, fmt.Errorf("decode dormant profiles: %w", err)
	}
	dormant := make([]*UserProfile, 0, len(candidates))
	for _, p := range candidates {
		if p.LastActiveAt.Before(cutoff) {
			dormant = append(dormant, p)
		}
	}
	return dormant, nil
}

// LapsedPaid returns every profile whose stored tier is paid but whose
// entitlement has lapsed at now — i.e. EffectiveTier(now) has collapsed to
// Free. Mirrors AdminStore.LapsedPaid exactly: load paid-tier candidates
// server-side (WHERE tier != 'Free'), then filter in Go with the domain rule
// so the expiry/grace-period logic is never re-expressed in SQL.
func (s *PostgresAdminStore) LapsedPaid(ctx context.Context, now time.Time) ([]*UserProfile, error) {
	rows, err := s.db.Query(ctx, pgAdminSelectCols+"WHERE tier != 'Free'")
	if err != nil {
		return nil, fmt.Errorf("query lapsed paid profiles: %w", err)
	}
	candidates, err := collectUsers(rows)
	if err != nil {
		return nil, fmt.Errorf("decode lapsed paid profiles: %w", err)
	}
	lapsed := make([]*UserProfile, 0)
	for _, p := range candidates {
		if p.Tier.IsPaid() && p.EffectiveTier(now) == TierFree {
			lapsed = append(lapsed, p)
		}
	}
	return lapsed, nil
}

// Save upserts the profile (same SQL as PostgresStore.Save). Used by the
// subscription sweep and admin grant endpoint as the write-back path.
func (s *PostgresAdminStore) Save(ctx context.Context, p *UserProfile) error {
	zonePrefText, err := marshalZonePrefs(p.ZonePreferences)
	if err != nil {
		return fmt.Errorf("encode zone_preferences for %q: %w", p.UserID, err)
	}
	emailDigest := p.Preferences.EmailDigestEnabled
	savedPush := p.Preferences.SavedDecisionPush
	savedEmail := p.Preferences.SavedDecisionEmail
	_, err = s.db.Exec(ctx, pgSaveUserQuery,
		p.UserID, p.Email, p.Preferences.PushEnabled, int(p.Preferences.DigestDay),
		&emailDigest, &savedPush, &savedEmail, zonePrefText,
		p.Tier.String(), p.SubscriptionExpiry, p.OriginalTransactionID, p.GracePeriodExpiry,
		p.LastActiveAt, p.LastActiveAt.UnixMilli(), p.WatchZoneCount,
	)
	if err != nil {
		return fmt.Errorf("upsert profile %q (admin): %w", p.UserID, err)
	}
	return nil
}

// listCursor is the keyset pagination cursor for List. It carries the
// (created_at, user_id) pair of the last row on the previous page — the same
// compound key List orders by — so paging is stable even when many rows share
// a created_at (the migration backfill gives every pre-existing row the same
// timestamp, so user_id is the essential tiebreak).
type listCursor struct {
	CreatedAt  time.Time
	LastUserID string
}

// encodeListCursor renders the cursor as base64url("<createdAt RFC3339Nano>|<userID>")
// so it is opaque to callers. created_at is encoded first: it never contains a
// "|", so decodeListCursor can split on the first separator and keep any "|" in
// the user id (auth0|..., apple|...) intact.
func encodeListCursor(createdAt time.Time, lastUserID string) string {
	raw := createdAt.UTC().Format(time.RFC3339Nano) + "|" + lastUserID
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeListCursor(token string) (listCursor, error) {
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return listCursor{}, fmt.Errorf("decode list cursor: %w", err)
	}
	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) != 2 {
		return listCursor{}, fmt.Errorf("decode list cursor: malformed cursor %q", string(b))
	}
	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return listCursor{}, fmt.Errorf("decode list cursor: parse created_at: %w", err)
	}
	return listCursor{CreatedAt: createdAt, LastUserID: parts[1]}, nil
}

// List returns one page of profiles ordered by (created_at, user_id) — oldest
// created first, so newly-created users naturally appear at the bottom of the
// list — optionally filtered by a case-insensitive email substring. An empty
// continuationToken starts from the first page; a non-empty token resumes after
// the (created_at, user_id) pair of the previous page's last row via a compound
// keyset guard. Mirrors AdminStore.List's contract (Page + opaque token).
func (s *PostgresAdminStore) List(ctx context.Context, emailSearch string, pageSize int, continuationToken string) (Page, error) {
	var cursor listCursor
	hasCursor := continuationToken != ""
	if hasCursor {
		var err error
		cursor, err = decodeListCursor(continuationToken)
		if err != nil {
			return Page{}, fmt.Errorf("list profiles: %w", err)
		}
	}

	// All four branches order by the compound (created_at, user_id) key; the two
	// cursor branches add the row-value keyset guard, shifting param numbering.
	const orderLimit = " ORDER BY created_at, user_id LIMIT "
	var (
		sql  string
		args []any
	)
	switch {
	case emailSearch != "" && hasCursor:
		sql = pgAdminSelectCols + "WHERE email ILIKE $1 AND (created_at, user_id) > ($2, $3)" + orderLimit + "$4"
		args = []any{"%" + emailSearch + "%", cursor.CreatedAt, cursor.LastUserID, pageSize}
	case emailSearch != "":
		sql = pgAdminSelectCols + "WHERE email ILIKE $1" + orderLimit + "$2"
		args = []any{"%" + emailSearch + "%", pageSize}
	case hasCursor:
		sql = pgAdminSelectCols + "WHERE (created_at, user_id) > ($1, $2)" + orderLimit + "$3"
		args = []any{cursor.CreatedAt, cursor.LastUserID, pageSize}
	default:
		sql = pgAdminSelectCols + strings.TrimPrefix(orderLimit, " ") + "$1"
		args = []any{pageSize}
	}

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return Page{}, fmt.Errorf("list profiles: %w", err)
	}
	profiles, err := collectUsers(rows)
	if err != nil {
		return Page{}, fmt.Errorf("list profiles: decode: %w", err)
	}

	// Emit a continuation token only when we got a full page — an undersized
	// page means we reached the end.
	nextToken := ""
	if len(profiles) == pageSize {
		last := profiles[len(profiles)-1]
		nextToken = encodeListCursor(last.CreatedAt, last.UserID)
	}

	return Page{Profiles: profiles, ContinuationToken: nextToken}, nil
}
