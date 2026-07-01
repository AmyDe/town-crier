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

// PaidCandidates loads every profile whose stored tier is paid (tier != 'Free'),
// unfiltered — the caller classifies each in Go via EffectiveTier(now). Unlike
// LapsedPaid (which filters to the lapsed subset), this returns ALL paid-tier
// candidates so the admin stats handler can bucket them into effective-paid /
// App Store / comped / lapsed / in-grace without re-expressing the expiry/grace
// rule in SQL. The candidate set is small (only paying users), so loading it
// whole is cheap.
func (s *PostgresAdminStore) PaidCandidates(ctx context.Context) ([]*UserProfile, error) {
	rows, err := s.db.Query(ctx, pgAdminSelectCols+"WHERE tier != 'Free'")
	if err != nil {
		return nil, fmt.Errorf("query paid candidates: %w", err)
	}
	candidates, err := collectUsers(rows)
	if err != nil {
		return nil, fmt.Errorf("decode paid candidates: %w", err)
	}
	return candidates, nil
}

// RecentSignup identifies the single most-recently-created user for the admin
// stats "signups" block. Email is nil when the account has none (e.g. a
// Sign-in-with-Apple user who withheld it).
type RecentSignup struct {
	UserID    string
	Email     *string
	CreatedAt time.Time
}

// UserStats is the admin-dashboard aggregate over the users table: whole-base
// counts computed in SQL (COUNT / GROUP BY / MAX) and returned as a plain
// struct. The paying breakdown is deliberately NOT here — it is classified in Go
// from PaidCandidates via EffectiveTier so the expiry/grace rule stays in the
// domain. now is injected by the caller (never SQL now()), matching the
// codebase's clock convention.
type UserStats struct {
	Total           int
	ByTier          map[string]int
	Signups24h      int
	Signups7d       int
	Signups30d      int
	MostRecent      *RecentSignup
	Active24h       int
	Active7d        int
	ZeroWatchZones  int
	NoEmail         int
	TotalWatchZones int
}

// pgUserStatsScalarQuery computes every scalar aggregate in one pass. The signup
// and activity windows compare against caller-supplied cutoffs ($1 = now-24h,
// $2 = now-7d, $3 = now-30d) rather than SQL now(). Column order MUST match the
// scan in UserStats.
const pgUserStatsScalarQuery = `
SELECT
    count(*),
    count(*) FILTER (WHERE created_at > $1),
    count(*) FILTER (WHERE created_at > $2),
    count(*) FILTER (WHERE created_at > $3),
    count(*) FILTER (WHERE last_active_at > $1),
    count(*) FILTER (WHERE last_active_at > $2),
    count(*) FILTER (WHERE COALESCE(watch_zone_count, 0) = 0),
    count(*) FILTER (WHERE email IS NULL),
    COALESCE(SUM(watch_zone_count), 0)
FROM users`

const pgUserStatsTierQuery = "SELECT tier, count(*) FROM users GROUP BY tier"

const pgUserStatsMostRecentQuery = "SELECT user_id, email, created_at FROM users ORDER BY created_at DESC LIMIT 1"

// UserStats returns the whole-user-base aggregate for GET /v1/admin/stats. It
// issues three read queries: the scalar aggregate (counts + total watch zones),
// the per-tier GROUP BY breakdown, and the single most-recent signup. now is the
// caller's clock; the 24h/7d/30d cutoffs are derived from it in Go so the SQL
// carries no now().
func (s *PostgresAdminStore) UserStats(ctx context.Context, now time.Time) (UserStats, error) {
	cutoff24h := now.Add(-24 * time.Hour)
	cutoff7d := now.Add(-7 * 24 * time.Hour)
	cutoff30d := now.Add(-30 * 24 * time.Hour)

	stats := UserStats{ByTier: map[string]int{"Free": 0, "Personal": 0, "Pro": 0}}

	scalarRows, err := s.db.Query(ctx, pgUserStatsScalarQuery, cutoff24h, cutoff7d, cutoff30d)
	if err != nil {
		return UserStats{}, fmt.Errorf("query user stats: %w", err)
	}
	scalar, err := pgx.CollectExactlyOneRow(scalarRows, func(row pgx.CollectableRow) (UserStats, error) {
		var us UserStats
		if scanErr := row.Scan(
			&us.Total,
			&us.Signups24h, &us.Signups7d, &us.Signups30d,
			&us.Active24h, &us.Active7d,
			&us.ZeroWatchZones, &us.NoEmail, &us.TotalWatchZones,
		); scanErr != nil {
			return UserStats{}, scanErr
		}
		return us, nil
	})
	if err != nil {
		return UserStats{}, fmt.Errorf("scan user stats: %w", err)
	}
	scalar.ByTier = stats.ByTier
	stats = scalar

	tierRows, err := s.db.Query(ctx, pgUserStatsTierQuery)
	if err != nil {
		return UserStats{}, fmt.Errorf("query user stats tiers: %w", err)
	}
	defer tierRows.Close()
	for tierRows.Next() {
		var (
			tier  string
			count int
		)
		if scanErr := tierRows.Scan(&tier, &count); scanErr != nil {
			return UserStats{}, fmt.Errorf("scan user stats tier: %w", scanErr)
		}
		stats.ByTier[tier] = count
	}
	if scanErr := tierRows.Err(); scanErr != nil {
		return UserStats{}, fmt.Errorf("user stats tier rows: %w", scanErr)
	}

	recent, err := s.mostRecentSignup(ctx)
	if err != nil {
		return UserStats{}, err
	}
	stats.MostRecent = recent
	return stats, nil
}

// mostRecentSignup reads the newest-created user for the stats "signups" block,
// returning nil (not an error) when the user base is empty.
func (s *PostgresAdminStore) mostRecentSignup(ctx context.Context) (*RecentSignup, error) {
	rows, err := s.db.Query(ctx, pgUserStatsMostRecentQuery)
	if err != nil {
		return nil, fmt.Errorf("query most recent signup: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if scanErr := rows.Err(); scanErr != nil {
			return nil, fmt.Errorf("most recent signup rows: %w", scanErr)
		}
		return nil, nil //nolint:nilnil // an empty user base has no most-recent signup
	}
	var r RecentSignup
	if scanErr := rows.Scan(&r.UserID, &r.Email, &r.CreatedAt); scanErr != nil {
		return nil, fmt.Errorf("scan most recent signup: %w", scanErr)
	}
	return &r, nil
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
