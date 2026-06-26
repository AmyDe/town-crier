package profiles

import (
	"context"
	"encoding/base64"
	"fmt"
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

// listCursor is the keyset pagination cursor for List. It encodes the last
// user_id seen on the previous page as a base64url string so it is opaque to
// callers — matching the Cosmos continuation-token contract.
type listCursor struct {
	LastUserID string
}

func encodeListCursor(lastUserID string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(lastUserID))
}

func decodeListCursor(token string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", fmt.Errorf("decode list cursor: %w", err)
	}
	return string(b), nil
}

// List returns one page of profiles ordered by user_id, optionally filtered
// by a case-insensitive email substring. An empty continuationToken starts
// from the first page; a non-empty token resumes after the last user_id from
// the previous page. Mirrors AdminStore.List's contract (Page + opaque token).
func (s *PostgresAdminStore) List(ctx context.Context, emailSearch string, pageSize int, continuationToken string) (Page, error) {
	var lastUserID string
	if continuationToken != "" {
		var err error
		lastUserID, err = decodeListCursor(continuationToken)
		if err != nil {
			return Page{}, fmt.Errorf("list profiles: %w", err)
		}
	}

	// Build the WHERE clause from the two optional predicates.
	// We always add at least the cursor guard when paging; on the first page
	// (empty cursor) we still ORDER BY user_id so the cursor is meaningful.
	var (
		sql  string
		args []any
	)
	switch {
	case emailSearch != "" && lastUserID != "":
		sql = pgAdminSelectCols + "WHERE email ILIKE $1 AND user_id > $2 ORDER BY user_id LIMIT $3"
		args = []any{"%" + emailSearch + "%", lastUserID, pageSize}
	case emailSearch != "":
		sql = pgAdminSelectCols + "WHERE email ILIKE $1 ORDER BY user_id LIMIT $2"
		args = []any{"%" + emailSearch + "%", pageSize}
	case lastUserID != "":
		sql = pgAdminSelectCols + "WHERE user_id > $1 ORDER BY user_id LIMIT $2"
		args = []any{lastUserID, pageSize}
	default:
		sql = pgAdminSelectCols + "ORDER BY user_id LIMIT $1"
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
		nextToken = encodeListCursor(profiles[len(profiles)-1].UserID)
	}

	return Page{Profiles: profiles, ContinuationToken: nextToken}, nil
}
