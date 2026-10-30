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

// Store is the full point-store method set *PostgresStore satisfies and the
// exported consumer-side interface the handlers and wiring depend on.
//
// The set covers:
//   - the profileStore interface in handler.go (Get/Save/Delete)
//   - the profileCAS interface in watchzones/handler.go (GetWithETag/UpdateZoneCountWithCAS)
//   - erasure.ProfileDeleter (Delete)
//   - middleware adapters NewTierLookup/NewActivityRecorder (Get/RecordActivity)
type Store interface {
	Get(ctx context.Context, userID string) (*UserProfile, error)
	Save(ctx context.Context, p *UserProfile) error
	RecordActivity(ctx context.Context, userID string, at time.Time) error
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
	ByDigestDay(ctx context.Context, day time.Weekday, notSentSince time.Time) ([]*UserProfile, error)
	MarkWeeklyDigestSent(ctx context.Context, userID string, at time.Time) error
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

// PostgresStore reads and writes user profiles in the Postgres `users` table.
// Cross-user scans live in PostgresAdminStore.
//
// CAS: the version column is a monotonic optimistic-lock counter shared by Save
// and UpdateZoneCountWithCAS. Both issue UPDATE ... WHERE user_id=$1 AND version=$n
// and increment version on success. Zero rows affected → platform.ErrCASPreconditionFailed.
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
	lifetime_tier, lifetime_original_transaction_id, lifetime_purchased_at, subscription_product_id,
	subscription_in_trial,
	last_active_at, last_active_at_epoch, created_at, watch_zone_count, version`

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
		lifetimeTier                 string
		email, originalTransactionID *string
		lifetimeOriginalTxnID        *string
		subscriptionProductID        *string
		subscriptionInTrial          bool
		pushEnabled                  bool
		digestDay                    int
		emailDigestEnabled           *bool
		savedDecisionPush            *bool
		savedDecisionEmail           *bool
		subscriptionExpiry           *time.Time
		gracePeriodExpiry            *time.Time
		lifetimePurchasedAt          *time.Time
		lastActiveAt                 time.Time
		lastActiveAtEpoch            int64
		createdAt                    time.Time
		watchZoneCount               *int
		version                      int
	)

	if err := row.Scan(
		&userID, &email, &pushEnabled, &digestDay,
		&emailDigestEnabled, &savedDecisionPush, &savedDecisionEmail,
		&zonePrefText,
		&tier, &subscriptionExpiry, &originalTransactionID, &gracePeriodExpiry,
		&lifetimeTier, &lifetimeOriginalTxnID, &lifetimePurchasedAt, &subscriptionProductID,
		&subscriptionInTrial,
		&lastActiveAt, &lastActiveAtEpoch, &createdAt, &watchZoneCount, &version,
	); err != nil {
		return nil, 0, err
	}

	t, err := ParseSubscriptionTier(tier)
	if err != nil {
		return nil, 0, fmt.Errorf("parse tier %q: %w", tier, err)
	}

	lt, err := ParseSubscriptionTier(lifetimeTier)
	if err != nil {
		return nil, 0, fmt.Errorf("parse lifetime tier %q: %w", lifetimeTier, err)
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
		ZonePreferences:               zones,
		Tier:                          t,
		SubscriptionExpiry:            subscriptionExpiry,
		OriginalTransactionID:         originalTransactionID,
		GracePeriodExpiry:             gracePeriodExpiry,
		LifetimeTier:                  lt,
		LifetimeOriginalTransactionID: lifetimeOriginalTxnID,
		LifetimePurchasedAt:           lifetimePurchasedAt,
		SubscriptionProductID:         subscriptionProductID,
		SubscriptionInTrial:           subscriptionInTrial,
		LastActiveAt:                  lastActiveAt,
		CreatedAt:                     createdAt,
		WatchZoneCount:                watchZoneCount,
		Version:                       version,
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

const pgInsertUserQuery = `
INSERT INTO users (
	user_id, email, push_enabled, digest_day,
	email_digest_enabled, saved_decision_push, saved_decision_email, zone_preferences,
	tier, subscription_expiry, original_transaction_id, grace_period_expiry,
	lifetime_tier, lifetime_original_transaction_id, lifetime_purchased_at, subscription_product_id,
	subscription_in_trial,
	last_active_at, last_active_at_epoch, watch_zone_count, version
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8::jsonb,
	$9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, 0
)`

const pgUpdateUserQuery = `
UPDATE users SET
	email                            = $2,
	push_enabled                     = $3,
	digest_day                       = $4,
	email_digest_enabled             = $5,
	saved_decision_push              = $6,
	saved_decision_email             = $7,
	zone_preferences                 = $8::jsonb,
	tier                             = $9,
	subscription_expiry              = $10,
	original_transaction_id          = $11,
	grace_period_expiry              = $12,
	lifetime_tier                    = $13,
	lifetime_original_transaction_id = $14,
	lifetime_purchased_at            = $15,
	subscription_product_id          = $16,
	subscription_in_trial            = $17,
	last_active_at                   = $18,
	last_active_at_epoch             = $19,
	watch_zone_count                 = $20,
	version                          = users.version + 1
WHERE user_id = $1 AND version = $21`

const (
	pgUniqueViolation          = "23505"
	pgOriginalTransactionIndex = "users_original_transaction_id_unique"
)

func isClaimedTransaction(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation &&
		pgErr.ConstraintName == pgOriginalTransactionIndex
}

// saveProfile inserts a profile built by NewProfile, or updates a loaded one
// only while its version is unchanged. A lost version check or a colliding
// insert returns platform.ErrCASPreconditionFailed.
func saveProfile(ctx context.Context, db querier, p *UserProfile) error {
	zonePrefText, err := marshalZonePrefs(p.ZonePreferences)
	if err != nil {
		return fmt.Errorf("encode zone_preferences for %q: %w", p.UserID, err)
	}
	emailDigest := p.Preferences.EmailDigestEnabled
	savedPush := p.Preferences.SavedDecisionPush
	savedEmail := p.Preferences.SavedDecisionEmail
	args := []any{
		p.UserID, p.Email, p.Preferences.PushEnabled, int(p.Preferences.DigestDay),
		&emailDigest, &savedPush, &savedEmail, zonePrefText,
		p.Tier.String(), p.SubscriptionExpiry, p.OriginalTransactionID, p.GracePeriodExpiry,
		p.LifetimeTier.String(), p.LifetimeOriginalTransactionID, p.LifetimePurchasedAt, p.SubscriptionProductID,
		p.SubscriptionInTrial,
		p.LastActiveAt, p.LastActiveAt.UnixMilli(), p.WatchZoneCount,
	}

	if p.IsNew {
		if _, err := db.Exec(ctx, pgInsertUserQuery, args...); err != nil {
			if isClaimedTransaction(err) {
				return fmt.Errorf("insert profile %q: %w", p.UserID, ErrTransactionClaimed)
			}
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
				return fmt.Errorf("insert profile %q: %w", p.UserID, platform.ErrCASPreconditionFailed)
			}
			return fmt.Errorf("insert profile %q: %w", p.UserID, err)
		}
		p.IsNew = false
		p.Version = 0
		return nil
	}

	tag, err := db.Exec(ctx, pgUpdateUserQuery, append(args, p.Version)...)
	if err != nil {
		if isClaimedTransaction(err) {
			return fmt.Errorf("update profile %q: %w", p.UserID, ErrTransactionClaimed)
		}
		return fmt.Errorf("update profile %q: %w", p.UserID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update profile %q: %w", p.UserID, platform.ErrCASPreconditionFailed)
	}
	p.Version++
	return nil
}

// Save inserts a new profile at version 0 or updates a loaded one when its
// Version still matches the stored row, incrementing the version. A stale
// Version, a deleted row or an insert that collides with an existing row
// returns platform.ErrCASPreconditionFailed. An original transaction id owned
// by another profile returns ErrTransactionClaimed.
func (s *PostgresStore) Save(ctx context.Context, p *UserProfile) error {
	return saveProfile(ctx, s.db, p)
}

const pgRecordActivityQuery = `
UPDATE users SET last_active_at = $2, last_active_at_epoch = $3
WHERE user_id = $1`

// RecordActivity sets only the activity columns, leaving every other column and
// the version untouched. An unknown user is a no-op.
func (s *PostgresStore) RecordActivity(ctx context.Context, userID string, at time.Time) error {
	if _, err := s.db.Exec(ctx, pgRecordActivityQuery, userID, at, at.UnixMilli()); err != nil {
		return fmt.Errorf("record activity for %q: %w", userID, err)
	}
	return nil
}

const pgDeleteUserQuery = "DELETE FROM users WHERE user_id = $1"

// Delete removes the profile document. Zero rows affected surfaces as
// ErrNotFound.
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
	email                            = $2,
	push_enabled                     = $3,
	digest_day                       = $4,
	email_digest_enabled             = $5,
	saved_decision_push              = $6,
	saved_decision_email             = $7,
	zone_preferences                 = $8::jsonb,
	tier                             = $9,
	subscription_expiry              = $10,
	original_transaction_id          = $11,
	grace_period_expiry              = $12,
	lifetime_tier                    = $13,
	lifetime_original_transaction_id = $14,
	lifetime_purchased_at            = $15,
	subscription_product_id          = $16,
	subscription_in_trial            = $17,
	last_active_at                   = $18,
	last_active_at_epoch             = $19,
	watch_zone_count                 = $20,
	version                          = version + 1
WHERE user_id = $1 AND version = $21`

// UpdateZoneCountWithCAS replaces the profile row only when the stored version
// matches the expected version encoded in etag. The version is atomically
// incremented on success.
//
// Zero rows affected (version guard fired — concurrent writer won) returns
// platform.ErrCASPreconditionFailed wrapped via %w.
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
		p.LifetimeTier.String(), p.LifetimeOriginalTransactionID, p.LifetimePurchasedAt, p.SubscriptionProductID,
		p.SubscriptionInTrial,
		p.LastActiveAt, p.LastActiveAt.UnixMilli(), p.WatchZoneCount,
		expectedVersion,
	)
	if err != nil {
		return fmt.Errorf("CAS update profile %q: %w", userID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("CAS update profile %q: %w", userID, platform.ErrCASPreconditionFailed)
	}
	p.Version = expectedVersion + 1
	return nil
}
