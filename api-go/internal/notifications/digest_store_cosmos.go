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
// cross-partition DISTINCT projection (the users with unsent emails), and an
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

// userIDsWithUnsentEmailsQuery projects the distinct user ids that have at least
// one unsent-email notification, across all partitions. Mirrors .NET
// GetUserIdsWithUnsentEmailsCrossPartitionAsync.
const userIDsWithUnsentEmailsQuery = "SELECT DISTINCT VALUE c.userId FROM c WHERE c.emailSent = false OR NOT IS_DEFINED(c.emailSent)"

// UserIDsWithUnsentEmails returns every user id with at least one unsent-email
// notification — the hourly cycle's candidate set before per-user tier gating.
func (s *DigestStore) UserIDsWithUnsentEmails(ctx context.Context) ([]string, error) {
	raws, err := s.items.QueryItemsCrossPartition(ctx, userIDsWithUnsentEmailsQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("query users with unsent emails: %w", err)
	}
	userIDs := make([]string, 0, len(raws))
	for _, raw := range raws {
		var id string
		if err := json.Unmarshal(raw, &id); err != nil {
			return nil, fmt.Errorf("decode user id: %w", err)
		}
		userIDs = append(userIDs, id)
	}
	return userIDs, nil
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
