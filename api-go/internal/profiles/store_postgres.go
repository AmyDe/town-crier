package profiles

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// querier is the consumer-side slice of *pgxpool.Pool the store uses:
// parameterised exec/query/query-row. Defining it here (not importing pgxpool)
// keeps the store decoupled from the concrete pool and lets a pgx.Tx stand in.
// Both *pgxpool.Pool and pgx.Tx satisfy it structurally.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// errCASPreconditionFailed is the sentinel returned by UpdateZoneCountWithCAS
// when the conditional UPDATE matches zero rows (the version guard fired,
// meaning a concurrent writer won the race). It is identical to
// platform.ErrCASPreconditionFailed so the watchzones quota CAS handler can
// branch on a single errors.Is check regardless of which backend is wired.
//
// Note: this is NOT a re-declaration. The Postgres store re-uses the platform
// sentinel directly so callers need not know which backend produced the error.
var errCASPreconditionFailed = platform.ErrCASPreconditionFailed

// Store is the full point-store method set *PostgresStore satisfies and the
// exported consumer-side interface the handlers and wiring depend on.
//
// The set covers:
//   - the profileStore interface in handler.go (Get/Save/Delete)
//   - the profileCAS interface in watchzones/handler.go (GetWithETag/UpdateZoneCountWithCAS)
//   - erasure.ProfileDeleter (Delete)
//   - middleware adapters NewTierLookup/NewActivityRecorder (Get/Save)
type Store interface {
	Get(ctx context.Context, userID string) (*UserProfile, error)
	Save(ctx context.Context, p *UserProfile) error
	Delete(ctx context.Context, userID string) error
	GetWithETag(ctx context.Context, userID string) (*UserProfile, string, error)
	UpdateZoneCountWithCAS(ctx context.Context, userID string, p *UserProfile, etag string) error
}

// AdminProfileStore is the full admin-store method set *PostgresAdminStore
// satisfies, covering the cross-user surface (find-by-email, digest-day and
// dormant scans, lapsed-paid sweep, save, paged list).
type AdminProfileStore interface {
	GetByEmail(ctx context.Context, email string) (*UserProfile, error)
	GetByOriginalTransactionID(ctx context.Context, originalTransactionID string) (*UserProfile, error)
	ByDigestDay(ctx context.Context, day time.Weekday) ([]*UserProfile, error)
	Dormant(ctx context.Context, cutoff time.Time) ([]*UserProfile, error)
	LapsedPaid(ctx context.Context, now time.Time) ([]*UserProfile, error)
	Save(ctx context.Context, p *UserProfile) error
	List(ctx context.Context, emailSearch string, pageSize int, continuationToken string) (Page, error)
}

// Compile-time check: the stores satisfy their exported interfaces.
var (
	_ Store = (*PostgresStore)(nil)

	_ AdminProfileStore = (*PostgresAdminStore)(nil)
)

// PostgresStore reads and writes user profiles in the Postgres `users` table
// (Cosmos → Postgres + PostGIS migration; memo 0010, epic #645). It is a
// parallel implementation: the Cosmos store remains wired until the backend
// flag flips, so nothing here is on a live path yet.
//
// Partition strategy: user_id is the natural primary key — every /v1/me
// operation is a single-row point read/write. Cross-user scans (digest sweep,
// dormant check) are in PostgresAdminStore.
//
// CAS: the version column is a monotonic optimistic-lock counter.
// UpdateZoneCountWithCAS issues UPDATE ... WHERE user_id=$1 AND version=$n and
// increments version on success. Zero rows affected → ErrCASPreconditionFailed.
type PostgresStore struct {
	db querier
}

// NewPostgresStore returns a store over the given pgx pool (or any querier).
func NewPostgresStore(db querier) *PostgresStore {
	return &PostgresStore{db: db}
}

// userSelectCols is the column list for SELECT queries. Order MUST match the
// scan destinations in scanUserRow.
const userSelectCols = `user_id, email, push_enabled, digest_day,
	email_digest_enabled, saved_decision_push, saved_decision_email,
	zone_preferences::text,
	tier, subscription_expiry, original_transaction_id, grace_period_expiry,
	last_active_at, last_active_at_epoch, watch_zone_count, version`

// rowScanner is the minimal interface that both pgx.Row (returned by QueryRow)
// and pgx.Rows (returned by Query, positioned via Next) satisfy. Defining it
// here lets scanUserRow serve both point-reads (PostgresStore.Get,
// GetWithETag) and collection scans (collectUsers in admin_store_postgres.go)
// without a concrete adapter type.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanUserRow scans one row from the users table into a UserProfile plus the
// raw version integer. zone_preferences is cast to text on the wire so pgx
// delivers it as a plain string we can unmarshal; all nullable columns scan
// into pointer types.
func scanUserRow(row rowScanner) (*UserProfile, int, error) {
	var (
		userID, tier, zonePrefText   string
		email, originalTransactionID *string
		pushEnabled                  bool
		digestDay                    int
		emailDigestEnabled           *bool
		savedDecisionPush            *bool
		savedDecisionEmail           *bool
		subscriptionExpiry           *time.Time
		gracePeriodExpiry            *time.Time
		lastActiveAt                 time.Time
		lastActiveAtEpoch            int64
		watchZoneCount               *int
		version                      int
	)

	if err := row.Scan(
		&userID, &email, &pushEnabled, &digestDay,
		&emailDigestEnabled, &savedDecisionPush, &savedDecisionEmail,
		&zonePrefText,
		&tier, &subscriptionExpiry, &originalTransactionID, &gracePeriodExpiry,
		&lastActiveAt, &lastActiveAtEpoch, &watchZoneCount, &version,
	); err != nil {
		return nil, 0, err
	}

	t, err := ParseSubscriptionTier(tier)
	if err != nil {
		return nil, 0, fmt.Errorf("parse tier %q: %w", tier, err)
	}

	zones, err := unmarshalZonePrefs(zonePrefText)
	if err != nil {
		return nil, 0, fmt.Errorf("unmarshal zone_preferences: %w", err)
	}

	p := &UserProfile{
		UserID: userID,
		Email:  email,
		Preferences: NotificationPreferences{
			PushEnabled:        pushEnabled,
			DigestDay:          time.Weekday(digestDay),
			EmailDigestEnabled: coalesceTrue(emailDigestEnabled),
			SavedDecisionPush:  coalesceTrue(savedDecisionPush),
			SavedDecisionEmail: coalesceTrue(savedDecisionEmail),
		},
		ZonePreferences:       zones,
		Tier:                  t,
		SubscriptionExpiry:    subscriptionExpiry,
		OriginalTransactionID: originalTransactionID,
		GracePeriodExpiry:     gracePeriodExpiry,
		LastActiveAt:          lastActiveAt,
		WatchZoneCount:        watchZoneCount,
	}
	return p, version, nil
}

// unmarshalZonePrefs decodes the zone_preferences JSON text from the database
// into the domain ZonePreferences map. An empty string (or "{}") produces an
// empty map, never nil.
func unmarshalZonePrefs(text string) (map[string]ZonePreferences, error) {
	zones := make(map[string]ZonePreferences)
	if text == "" || text == "{}" {
		return zones, nil
	}
	var docs map[string]zonePreferencesDocument
	if err := json.Unmarshal([]byte(text), &docs); err != nil {
		return nil, err
	}
	for id, d := range docs {
		zones[id] = ZonePreferences(d)
	}
	return zones, nil
}

// marshalZonePrefs encodes the domain ZonePreferences map to a JSON string
// suitable for passing as a parameter with ::jsonb cast.
func marshalZonePrefs(zones map[string]ZonePreferences) (string, error) {
	if len(zones) == 0 {
		return "{}", nil
	}
	docs := make(map[string]zonePreferencesDocument, len(zones))
	for id, z := range zones {
		docs[id] = zonePreferencesDocument(z)
	}
	b, err := json.Marshal(docs)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

const pgGetUserQuery = "SELECT " + userSelectCols + " FROM users WHERE user_id = $1"

// Get point-reads the profile for userID. A miss surfaces as ErrNotFound;
// any other failure is wrapped and returned.
func (s *PostgresStore) Get(ctx context.Context, userID string) (*UserProfile, error) {
	p, _, err := scanUserRow(s.db.QueryRow(ctx, pgGetUserQuery, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read profile %q: %w", userID, err)
	}
	return p, nil
}

const pgSaveUserQuery = `
INSERT INTO users (
	user_id, email, push_enabled, digest_day,
	email_digest_enabled, saved_decision_push, saved_decision_email, zone_preferences,
	tier, subscription_expiry, original_transaction_id, grace_period_expiry,
	last_active_at, last_active_at_epoch, watch_zone_count, version
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8::jsonb,
	$9, $10, $11, $12, $13, $14, $15, 0
)
ON CONFLICT (user_id) DO UPDATE SET
	email                   = EXCLUDED.email,
	push_enabled            = EXCLUDED.push_enabled,
	digest_day              = EXCLUDED.digest_day,
	email_digest_enabled    = EXCLUDED.email_digest_enabled,
	saved_decision_push     = EXCLUDED.saved_decision_push,
	saved_decision_email    = EXCLUDED.saved_decision_email,
	zone_preferences        = EXCLUDED.zone_preferences,
	tier                    = EXCLUDED.tier,
	subscription_expiry     = EXCLUDED.subscription_expiry,
	original_transaction_id = EXCLUDED.original_transaction_id,
	grace_period_expiry     = EXCLUDED.grace_period_expiry,
	last_active_at          = EXCLUDED.last_active_at,
	last_active_at_epoch    = EXCLUDED.last_active_at_epoch,
	watch_zone_count        = EXCLUDED.watch_zone_count`

// Save upserts the profile. On conflict (existing user) the version column is
// deliberately NOT updated — only UpdateZoneCountWithCAS advances the version.
// A new row starts at version 0 (the column default).
func (s *PostgresStore) Save(ctx context.Context, p *UserProfile) error {
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
		return fmt.Errorf("upsert profile %q: %w", p.UserID, err)
	}
	return nil
}

const pgDeleteUserQuery = "DELETE FROM users WHERE user_id = $1"

// Delete removes the profile document. Zero rows affected surfaces as
// ErrNotFound, matching the Cosmos store's contract (callers may treat a
// missing profile as tolerable or fatal depending on context).
func (s *PostgresStore) Delete(ctx context.Context, userID string) error {
	tag, err := s.db.Exec(ctx, pgDeleteUserQuery, userID)
	if err != nil {
		return fmt.Errorf("delete profile %q: %w", userID, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetWithETag reads the profile and its current optimistic version as an etag
// string. Returns (nil, "", nil) when the profile does not exist — the
// watchzones quota CAS caller treats an absent profile as a benign no-op.
func (s *PostgresStore) GetWithETag(ctx context.Context, userID string) (*UserProfile, string, error) {
	p, version, err := scanUserRow(s.db.QueryRow(ctx, pgGetUserQuery, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("read profile %q with etag: %w", userID, err)
	}
	return p, strconv.Itoa(version), nil
}

const pgUpdateZoneCountCASQuery = `
UPDATE users SET
	email                   = $2,
	push_enabled            = $3,
	digest_day              = $4,
	email_digest_enabled    = $5,
	saved_decision_push     = $6,
	saved_decision_email    = $7,
	zone_preferences        = $8::jsonb,
	tier                    = $9,
	subscription_expiry     = $10,
	original_transaction_id = $11,
	grace_period_expiry     = $12,
	last_active_at          = $13,
	last_active_at_epoch    = $14,
	watch_zone_count        = $15,
	version                 = version + 1
WHERE user_id = $1 AND version = $16`

// UpdateZoneCountWithCAS replaces the profile row only when the stored version
// matches the expected version encoded in etag. The version is atomically
// incremented on success.
//
// Zero rows affected (version guard fired — concurrent writer won) returns
// platform.ErrCASPreconditionFailed wrapped via %w, identical to the Cosmos
// store's contract so the watchzones quota handler can branch on a single
// errors.Is check regardless of which backend is active.
func (s *PostgresStore) UpdateZoneCountWithCAS(ctx context.Context, userID string, p *UserProfile, etag string) error {
	expectedVersion, err := strconv.Atoi(etag)
	if err != nil {
		return fmt.Errorf("parse CAS etag %q for profile %q: %w", etag, userID, err)
	}

	zonePrefText, err := marshalZonePrefs(p.ZonePreferences)
	if err != nil {
		return fmt.Errorf("encode zone_preferences for CAS update of %q: %w", userID, err)
	}
	emailDigest := p.Preferences.EmailDigestEnabled
	savedPush := p.Preferences.SavedDecisionPush
	savedEmail := p.Preferences.SavedDecisionEmail

	tag, err := s.db.Exec(ctx, pgUpdateZoneCountCASQuery,
		userID,
		p.Email, p.Preferences.PushEnabled, int(p.Preferences.DigestDay),
		&emailDigest, &savedPush, &savedEmail, zonePrefText,
		p.Tier.String(), p.SubscriptionExpiry, p.OriginalTransactionID, p.GracePeriodExpiry,
		p.LastActiveAt, p.LastActiveAt.UnixMilli(), p.WatchZoneCount,
		expectedVersion,
	)
	if err != nil {
		return fmt.Errorf("CAS update profile %q: %w", userID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("CAS update profile %q: %w", userID, errCASPreconditionFailed)
	}
	return nil
}
