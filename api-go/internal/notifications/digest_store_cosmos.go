package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// DigestItems is the consumer-side slice of the Notifications container the
// digest store uses: a single-partition query (a user's recent / unsent rows), a
// cross-partition projection with client-side dedup (the users with unsent
// emails — azcosmos cannot serve a cross-partition DISTINCT, tc-b7cm), and an
// upsert (writing back the emailSent flag). platform.CosmosContainer satisfies
// it structurally.
type DigestItems interface {
	QueryItems(ctx context.Context, partitionKey, query string, params map[string]any) ([][]byte, error)
	QueryItemsCrossPartition(ctx context.Context, query string, params map[string]any) ([][]byte, error)
	UpsertItem(ctx context.Context, partitionKey string, item []byte) error
}

// DigestStore reads and writes the full Notifications-container documents the
// digest worker needs. It is separate from CosmosStore (the web API's
// latest-unread projection) because it hydrates the complete document and writes
// the emailSent flag back.
//
// Partition strategy: the container is partitioned by /userId. The per-user
// reads target one partition; the unsent-email user list is the only
// cross-partition scan, mirroring .NET's GetUserIdsWithUnsentEmailsCrossPartition.
type DigestStore struct {
	items DigestItems
}

// NewDigestStore returns a digest store backed by the given Cosmos item accessor.
func NewDigestStore(items DigestItems) *DigestStore {
	return &DigestStore{items: items}
}

// byUserSinceQuery selects a user's notifications created at or after a cutoff,
// newest first — the weekly-digest window. Mirrors .NET GetByUserSinceAsync.
const byUserSinceQuery = "SELECT * FROM c WHERE c.userId = @userId AND c.createdAt >= @since ORDER BY c.createdAt DESC"

// ByUserSince returns the user's notifications created at or after since, used by
// the weekly digest to gather the past week's activity.
func (s *DigestStore) ByUserSince(ctx context.Context, userID string, since time.Time) ([]DigestNotification, error) {
	raws, err := s.items.QueryItems(ctx, userID, byUserSinceQuery, map[string]any{
		"@userId": userID,
		"@since":  platform.DotNetTime(since).String(),
	})
	if err != nil {
		return nil, fmt.Errorf("query notifications since for %q: %w", userID, err)
	}
	return decodeDigests(raws, userID)
}

// allByUserQuery selects every notification in a user's partition, oldest first,
// with no since-floor — the GDPR-export read. It is single-partition (scoped to
// userId) so it never fans out cross-partition; the Notifications container's
// 90-day TTL naturally bounds the row count. The ORDER BY makes the export's
// notifications array deterministic so successive exports stay byte-stable.
const allByUserQuery = "SELECT * FROM c WHERE c.userId = @userId ORDER BY c.createdAt ASC"

// AllByUser returns every notification in the user's partition, hydrated to the
// full DigestNotification, for the GDPR data export (GET /v1/me/data). Unlike
// ByUserSince it applies no time window — the export covers the user's whole
// (TTL-bounded) notification history. Single-partition, never cross-partition.
func (s *DigestStore) AllByUser(ctx context.Context, userID string) ([]DigestNotification, error) {
	raws, err := s.items.QueryItems(ctx, userID, allByUserQuery, map[string]any{"@userId": userID})
	if err != nil {
		return nil, fmt.Errorf("query all notifications for %q: %w", userID, err)
	}
	return decodeDigests(raws, userID)
}

// unsentEmailsQuery selects a user's notifications whose email has not yet been
// sent, oldest first. The OR-NOT-IS_DEFINED clause includes legacy rows written
// before the emailSent field existed. Mirrors .NET GetUnsentEmailsByUserAsync.
const unsentEmailsQuery = "SELECT * FROM c WHERE c.userId = @userId AND (c.emailSent = false OR NOT IS_DEFINED(c.emailSent)) ORDER BY c.createdAt ASC"

// UnsentEmailsByUser returns the user's notifications awaiting an email, used by
// the hourly digest to gather the rows to render and then mark sent.
func (s *DigestStore) UnsentEmailsByUser(ctx context.Context, userID string) ([]DigestNotification, error) {
	raws, err := s.items.QueryItems(ctx, userID, unsentEmailsQuery, map[string]any{"@userId": userID})
	if err != nil {
		return nil, fmt.Errorf("query unsent emails for %q: %w", userID, err)
	}
	return decodeDigests(raws, userID)
}

// userIDsWithUnsentEmailsQuery projects the user ids that have at least one
// unsent-email notification, across all partitions. It is a cross-partition
// projection with client-side dedup (azcosmos cannot serve a cross-partition
// DISTINCT — the gateway returns 400 "can not be directly served by the
// gateway"; tc-b7cm). The same row's userId repeats once per unsent
// notification, so UserIDsWithUnsentEmails collapses them in Go. Mirrors .NET
// GetUserIdsWithUnsentEmailsCrossPartitionAsync.
const userIDsWithUnsentEmailsQuery = "SELECT VALUE c.userId FROM c WHERE c.emailSent = false OR NOT IS_DEFINED(c.emailSent)"

// UserIDsWithUnsentEmails returns every user id with at least one unsent-email
// notification — the hourly cycle's candidate set before per-user tier gating.
// The cross-partition projection returns one row per unsent notification, so the
// ids are de-duplicated here (a cross-partition DISTINCT 400s at the gateway,
// tc-b7cm). First-seen order is preserved.
func (s *DigestStore) UserIDsWithUnsentEmails(ctx context.Context) ([]string, error) {
	raws, err := s.items.QueryItemsCrossPartition(ctx, userIDsWithUnsentEmailsQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("query users with unsent emails: %w", err)
	}
	userIDs := make([]string, 0, len(raws))
	seen := make(map[string]struct{}, len(raws))
	for _, raw := range raws {
		var id string
		if err := json.Unmarshal(raw, &id); err != nil {
			return nil, fmt.Errorf("decode user id: %w", err)
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		userIDs = append(userIDs, id)
	}
	return userIDs, nil
}

// Create writes a freshly dispatched notification into the user's partition. It
// is the poll-path enqueuer's write primitive (epic tc-wad3, bead tc-uc2p): the
// document it writes is the full digestDocument shape — camelCase keys, the
// 90-day TTL, emailSent=false — so the digest worker's ByUserSince /
// UnsentEmailsByUser reads hydrate it unchanged. Mirrors .NET
// CosmosNotificationRepository.SaveAsync for a new notification.
func (s *DigestStore) Create(ctx context.Context, n DigestNotification) error {
	body, err := json.Marshal(newDigestDocument(n))
	if err != nil {
		return fmt.Errorf("encode notification %q: %w", n.ID, err)
	}
	if err := s.items.UpsertItem(ctx, n.UserID, body); err != nil {
		return fmt.Errorf("upsert notification %q: %w", n.ID, err)
	}
	return nil
}

// getByUserAndApplicationQuery is the dedup lookup: the user's notification (if
// any) for a given (applicationUid, authorityId, eventType). Authority is part
// of the key because PlanIt uids collide across councils (tc-th98 / GH#384), so
// a uid-only match would suppress a legitimate notification for a same-uid
// application in a different authority. Mirrors .NET GetByUserAndApplicationAsync.
const getByUserAndApplicationQuery = "SELECT * FROM c WHERE c.userId = @userId " +
	"AND c.applicationUid = @applicationUid AND c.authorityId = @authorityId " +
	"AND c.eventType = @eventType"

// GetByUserAndApplication returns the user's existing notification for the
// (applicationUid, authorityId, eventType) tuple, or nil when none exists — the
// "not yet notified" signal the enqueuer and decision dispatcher branch on for
// idempotency. The read is single-partition (scoped to userId), mirroring .NET.
func (s *DigestStore) GetByUserAndApplication(ctx context.Context, userID, applicationUID string, authorityID int, eventType EventType) (*DigestNotification, error) {
	raws, err := s.items.QueryItems(ctx, userID, getByUserAndApplicationQuery, map[string]any{
		"@userId":         userID,
		"@applicationUid": applicationUID,
		"@authorityId":    authorityID,
		"@eventType":      string(eventType),
	})
	if err != nil {
		return nil, fmt.Errorf("query existing notification for %q: %w", userID, err)
	}
	if len(raws) == 0 {
		return nil, nil //nolint:nilnil // absent notification is the "not yet notified" signal, not an error
	}
	var doc digestDocument
	if err := json.Unmarshal(raws[0], &doc); err != nil {
		return nil, fmt.Errorf("decode existing notification for %q: %w", userID, err)
	}
	n := doc.toDigest()
	return &n, nil
}

// MarkEmailSent upserts the notification document (with EmailSent already set by
// the caller) so it is excluded from the next hourly cycle, mirroring .NET's
// MarkEmailSent + SaveAsync.
func (s *DigestStore) MarkEmailSent(ctx context.Context, n DigestNotification) error {
	body, err := json.Marshal(newDigestDocument(n))
	if err != nil {
		return fmt.Errorf("encode notification %q: %w", n.ID, err)
	}
	if err := s.items.UpsertItem(ctx, n.UserID, body); err != nil {
		return fmt.Errorf("upsert notification %q: %w", n.ID, err)
	}
	return nil
}

// decodeDigests hydrates a batch of stored documents into the digest model.
func decodeDigests(raws [][]byte, userID string) ([]DigestNotification, error) {
	out := make([]DigestNotification, 0, len(raws))
	for _, raw := range raws {
		var doc digestDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("decode notification for %q: %w", userID, err)
		}
		out = append(out, doc.toDigest())
	}
	return out, nil
}
