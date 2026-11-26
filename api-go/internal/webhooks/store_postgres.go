package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
)

var (
	// ErrNotFound reports that the endpoint does not exist.
	ErrNotFound = errors.New("webhooks: endpoint not found")
	// ErrEndpointExists reports that the user already has an endpoint.
	ErrEndpointExists = errors.New("webhooks: endpoint already exists")
)

const pgUniqueViolationCode = "23505"

type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type querier interface {
	execer
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Delivery is one queued alert for an endpoint. ID is the public event id and
// Payload is the JSON body fixed when the alert was queued.
type Delivery struct {
	ID             string
	EndpointID     string
	NotificationID string
	EventType      EventType
	Payload        []byte
	Attempts       int
	CreatedAt      time.Time
}

// PostgresStore reads and writes the webhook_endpoints and webhook_deliveries
// tables.
type PostgresStore struct {
	db querier
}

// NewPostgresStore returns a store over the given pgx pool (or any querier).
func NewPostgresStore(db querier) *PostgresStore {
	return &PostgresStore{db: db}
}

const endpointColumns = "id, user_id, url, format, secret, status, disabled_reason, consecutive_failures, " +
	"last_success_at, last_failure_at, last_failure_reason, last_failure_status, last_test_at, created_at, updated_at"

func scanEndpoint(row pgx.CollectableRow) (Endpoint, error) {
	var e Endpoint
	if err := row.Scan(
		&e.ID, &e.UserID, &e.URL, &e.Format, &e.Secret, &e.Status, &e.DisabledReason, &e.ConsecutiveFailures,
		&e.LastSuccessAt, &e.LastFailureAt, &e.LastFailureReason, &e.LastFailureStatus, &e.LastTestAt,
		&e.CreatedAt, &e.UpdatedAt,
	); err != nil {
		return Endpoint{}, err
	}
	e.CreatedAt, e.UpdatedAt = e.CreatedAt.UTC(), e.UpdatedAt.UTC()
	for _, t := range []*time.Time{e.LastSuccessAt, e.LastFailureAt, e.LastTestAt} {
		if t != nil {
			*t = t.UTC()
		}
	}
	return e, nil
}

// oneEndpoint returns the single row of a query, or ErrNotFound for none.
func (s *PostgresStore) oneEndpoint(ctx context.Context, sql string, args ...any) (Endpoint, error) {
	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return Endpoint{}, err
	}
	e, err := pgx.CollectExactlyOneRow(rows, scanEndpoint)
	if errors.Is(err, pgx.ErrNoRows) {
		return Endpoint{}, ErrNotFound
	}
	return e, err
}

// ByUserID returns the user's endpoint, or ErrNotFound.
func (s *PostgresStore) ByUserID(ctx context.Context, userID string) (Endpoint, error) {
	e, err := s.oneEndpoint(ctx, "SELECT "+endpointColumns+" FROM webhook_endpoints WHERE user_id = $1", userID)
	if err != nil {
		return Endpoint{}, fmt.Errorf("read webhook endpoint for %q: %w", userID, err)
	}
	return e, nil
}

const pgCreateEndpointQuery = "INSERT INTO webhook_endpoints (" + endpointColumns + ") " +
	"VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)"

// Create stores a new endpoint. It returns ErrEndpointExists when the user
// already has one.
func (s *PostgresStore) Create(ctx context.Context, e Endpoint) error {
	_, err := s.db.Exec(ctx, pgCreateEndpointQuery,
		e.ID, e.UserID, e.URL, e.Format, e.Secret, e.Status, e.DisabledReason, e.ConsecutiveFailures,
		e.LastSuccessAt, e.LastFailureAt, e.LastFailureReason, e.LastFailureStatus, e.LastTestAt,
		e.CreatedAt, e.UpdatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolationCode {
		return ErrEndpointExists
	}
	if err != nil {
		return fmt.Errorf("create webhook endpoint %q: %w", e.ID, err)
	}
	return nil
}

const pgReplaceEndpointQuery = "UPDATE webhook_endpoints SET url = $2, format = $3, status = 'active', " +
	"disabled_reason = NULL, consecutive_failures = 0, updated_at = $4 WHERE user_id = $1 RETURNING " + endpointColumns

// Replace changes the URL and format of the user's endpoint, switches it on
// again and clears its failure count. The secret is kept. It returns
// ErrNotFound, ErrInvalidURL or ErrInvalidFormat.
func (s *PostgresStore) Replace(ctx context.Context, userID, rawURL string, format Format, now time.Time) (Endpoint, error) {
	if err := validateConfig(rawURL, format); err != nil {
		return Endpoint{}, err
	}
	e, err := s.oneEndpoint(ctx, pgReplaceEndpointQuery, userID, rawURL, format, now)
	if err != nil {
		return Endpoint{}, fmt.Errorf("replace webhook endpoint for %q: %w", userID, err)
	}
	return e, nil
}

// RotateSecret replaces the signing secret. It returns ErrNotFound when the
// user has no endpoint.
func (s *PostgresStore) RotateSecret(ctx context.Context, userID, secret string, now time.Time) error {
	tag, err := s.db.Exec(ctx,
		"UPDATE webhook_endpoints SET secret = $2, updated_at = $3 WHERE user_id = $1", userID, secret, now)
	if err != nil {
		return fmt.Errorf("rotate webhook secret for %q: %w", userID, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

const pgClaimTestQuery = "UPDATE webhook_endpoints SET last_test_at = $2 " +
	"WHERE user_id = $1 AND (last_test_at IS NULL OR last_test_at <= $3)"

// ClaimTest sets last_test_at to now and reports true, unless the user's last
// test was less than TestCooldown ago or the user has no endpoint. The check
// and the write are one statement, so concurrent API replicas cannot both pass.
// No other column changes.
func (s *PostgresStore) ClaimTest(ctx context.Context, userID string, now time.Time) (bool, error) {
	tag, err := s.db.Exec(ctx, pgClaimTestQuery, userID, now, now.Add(-TestCooldown))
	if err != nil {
		return false, fmt.Errorf("claim webhook test for %q: %w", userID, err)
	}
	return tag.RowsAffected() > 0, nil
}

// DeleteAllByUserID removes the user's endpoint and, by cascade, its
// deliveries. Deleting nothing is not an error.
func (s *PostgresStore) DeleteAllByUserID(ctx context.Context, userID string) error {
	if _, err := s.db.Exec(ctx, "DELETE FROM webhook_endpoints WHERE user_id = $1", userID); err != nil {
		return fmt.Errorf("delete webhook endpoint for %q: %w", userID, err)
	}
	return nil
}

const pgActiveEndpointsWithPendingQuery = "SELECT " + endpointColumns + " FROM webhook_endpoints e " +
	"WHERE e.status = 'active' AND EXISTS (SELECT 1 FROM webhook_deliveries d " +
	"WHERE d.endpoint_id = e.id AND d.status = 'pending') ORDER BY e.created_at, e.id"

// ActiveEndpointsWithPending returns every active endpoint that has at least
// one pending delivery.
func (s *PostgresStore) ActiveEndpointsWithPending(ctx context.Context) ([]Endpoint, error) {
	rows, err := s.db.Query(ctx, pgActiveEndpointsWithPendingQuery)
	if err != nil {
		return nil, fmt.Errorf("query webhook endpoints with pending deliveries: %w", err)
	}
	out, err := pgx.CollectRows(rows, scanEndpoint)
	if err != nil {
		return nil, fmt.Errorf("scan webhook endpoints with pending deliveries: %w", err)
	}
	return out, nil
}

const pgPendingDeliveriesQuery = "SELECT id, endpoint_id, notification_id, event_type, payload, attempts, created_at " +
	"FROM webhook_deliveries WHERE endpoint_id = $1 AND status = 'pending' ORDER BY created_at, id LIMIT $2"

// PendingDeliveries returns up to limit pending deliveries of the endpoint,
// oldest first.
func (s *PostgresStore) PendingDeliveries(ctx context.Context, endpointID string, limit int) ([]Delivery, error) {
	rows, err := s.db.Query(ctx, pgPendingDeliveriesQuery, endpointID, limit)
	if err != nil {
		return nil, fmt.Errorf("query pending deliveries of webhook %q: %w", endpointID, err)
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Delivery, error) {
		var d Delivery
		err := row.Scan(&d.ID, &d.EndpointID, &d.NotificationID, &d.EventType, &d.Payload, &d.Attempts, &d.CreatedAt)
		d.CreatedAt = d.CreatedAt.UTC()
		return d, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan pending deliveries of webhook %q: %w", endpointID, err)
	}
	return out, nil
}

const pgMarkDeliveredQuery = "UPDATE webhook_deliveries SET status = 'delivered', attempts = attempts + 1, " +
	"last_attempt_at = $2, delivered_at = $2 WHERE id = ANY($1) AND status = 'pending'"

// MarkDelivered records a successful attempt on the still-pending deliveries
// among ids.
func (s *PostgresStore) MarkDelivered(ctx context.Context, ids []string, at time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	if _, err := s.db.Exec(ctx, pgMarkDeliveredQuery, ids, at); err != nil {
		return fmt.Errorf("mark %d webhook deliveries delivered: %w", len(ids), err)
	}
	return nil
}

const pgRecordFailedAttemptQuery = `
WITH attempted AS (
    UPDATE webhook_deliveries
    SET attempts = attempts + 1,
        last_attempt_at = $2,
        status = CASE WHEN attempts + 1 >= $3 THEN 'failed' ELSE status END
    WHERE id = ANY($1) AND status = 'pending'
    RETURNING status
)
SELECT count(*) FROM attempted WHERE status = 'failed'`

// RecordFailedAttempt counts a failed attempt on the still-pending deliveries
// among ids. Those that reach MaxDeliveryAttempts become failed, and it
// returns how many did.
func (s *PostgresStore) RecordFailedAttempt(ctx context.Context, ids []string, at time.Time) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	rows, err := s.db.Query(ctx, pgRecordFailedAttemptQuery, ids, at, MaxDeliveryAttempts)
	if err != nil {
		return 0, fmt.Errorf("record failed webhook attempt: %w", err)
	}
	failed, err := pgx.CollectExactlyOneRow(rows, pgx.RowTo[int])
	if err != nil {
		return 0, fmt.Errorf("scan failed webhook attempt: %w", err)
	}
	return failed, nil
}

// RecordEndpointSuccess clears the failure count after a run that delivered
// something. url is the one the run sent to: when the endpoint no longer has it,
// nothing changes.
func (s *PostgresStore) RecordEndpointSuccess(ctx context.Context, endpointID, url string, at time.Time) error {
	if _, err := s.db.Exec(ctx,
		"UPDATE webhook_endpoints SET consecutive_failures = 0, last_success_at = $3, updated_at = $3 "+
			"WHERE id = $1 AND url = $2",
		endpointID, url, at); err != nil {
		return fmt.Errorf("record success of webhook %q: %w", endpointID, err)
	}
	return nil
}

const pgRecordEndpointFailureQuery = "UPDATE webhook_endpoints SET consecutive_failures = consecutive_failures + 1, " +
	"last_failure_at = $3, last_failure_reason = $4, last_failure_status = $5, updated_at = $3 " +
	"WHERE id = $1 AND url = $2 RETURNING consecutive_failures"

// RecordEndpointFailure counts one failed run and returns the new consecutive
// failure count. It returns ErrNotFound, and changes nothing, when no endpoint
// has both endpointID and the url the run sent to. A zero result.Status is
// stored as null.
func (s *PostgresStore) RecordEndpointFailure(ctx context.Context, endpointID, url string, result Result, at time.Time) (int, error) {
	var status *int
	if result.Status != 0 {
		status = &result.Status
	}
	rows, err := s.db.Query(ctx, pgRecordEndpointFailureQuery, endpointID, url, at, result.Failure, status)
	if err != nil {
		return 0, fmt.Errorf("record failure of webhook %q: %w", endpointID, err)
	}
	count, err := pgx.CollectExactlyOneRow(rows, pgx.RowTo[int])
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("scan failure of webhook %q: %w", endpointID, err)
	}
	return count, nil
}

const pgDisableEndpointQuery = `
WITH disabled AS (
    UPDATE webhook_endpoints
    SET status = 'disabled', disabled_reason = $3, updated_at = $4
    WHERE id = $1 AND url = $2 AND status = 'active'
    RETURNING id
), dropped AS (
    UPDATE webhook_deliveries SET status = 'failed'
    WHERE endpoint_id IN (SELECT id FROM disabled) AND status = 'pending'
)
SELECT count(*) FROM disabled`

// DisableEndpoint switches off an active endpoint that still has url and, in the
// same statement, fails all its pending deliveries. It reports true only when
// this call switched it off.
func (s *PostgresStore) DisableEndpoint(ctx context.Context, endpointID, url string, reason DisabledReason, at time.Time) (bool, error) {
	rows, err := s.db.Query(ctx, pgDisableEndpointQuery, endpointID, url, reason, at)
	if err != nil {
		return false, fmt.Errorf("disable webhook %q: %w", endpointID, err)
	}
	count, err := pgx.CollectExactlyOneRow(rows, pgx.RowTo[int])
	if err != nil {
		return false, fmt.Errorf("scan disable of webhook %q: %w", endpointID, err)
	}
	return count > 0, nil
}

// PurgeDeliveriesOlderThan deletes deliveries of any status created before
// cutoff and returns how many it deleted.
func (s *PostgresStore) PurgeDeliveriesOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := s.db.Exec(ctx, "DELETE FROM webhook_deliveries WHERE created_at < $1", cutoff)
	if err != nil {
		return 0, fmt.Errorf("purge webhook deliveries: %w", err)
	}
	return tag.RowsAffected(), nil
}

type slugResolver interface {
	SlugForAreaID(id int) (string, bool)
}

// Outbox queues alerts for later delivery.
type Outbox struct {
	db    execer
	slugs slugResolver
	newID func() string
	now   func() time.Time
}

// NewOutbox returns an outbox. newID must return a new UUID, which becomes the
// delivery id and the public event id.
func NewOutbox(db execer, slugs slugResolver, newID func() string, now func() time.Time) *Outbox {
	return &Outbox{db: db, slugs: slugs, newID: newID, now: now}
}

const pgEnqueueDeliveryQuery = `
INSERT INTO webhook_deliveries (id, endpoint_id, notification_id, event_type, payload, status, attempts, created_at)
SELECT $1, e.id, $2, $3, $4, 'pending', 0, $5
FROM webhook_endpoints e
WHERE e.user_id = $6 AND e.status = 'active'
ON CONFLICT (endpoint_id, notification_id) DO NOTHING`

// Enqueue queues one notification record for its user's active endpoint. With
// no active endpoint, or when the record is already queued, it does nothing and
// returns nil. It does not check the user's tier. zoneName is "" for a record
// with no watch zone.
func (o *Outbox) Enqueue(ctx context.Context, n notifications.DigestNotification, app applications.PlanningApplication, zoneName string) error {
	id := o.newID()
	slug, _ := o.slugs.SlugForAreaID(app.AreaID)
	payload, err := BuildPayload(id, n, app, zoneName, slug)
	if err != nil {
		return err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload for notification %q: %w", n.ID, err)
	}
	if _, err := o.db.Exec(ctx, pgEnqueueDeliveryQuery,
		id, n.ID, payload.Type, json.RawMessage(body), o.now().UTC(), n.UserID); err != nil {
		return fmt.Errorf("queue webhook delivery for notification %q: %w", n.ID, err)
	}
	return nil
}
