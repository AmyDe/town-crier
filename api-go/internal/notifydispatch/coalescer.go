package notifydispatch

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// zoneNameReader resolves a user's watch zones so the coalescer can name a
// zone's summary push at flush time. watchzones.Store satisfies it — the same
// GetByUserID pattern the digest handler already uses.
type zoneNameReader interface {
	GetByUserID(ctx context.Context, userID string) ([]watchzones.WatchZone, error)
}

// savedBucket is the accumulator key for zone-less (saved) notifications. No
// real watch zone id is the empty string, so it never collides with one.
const savedBucket = ""

// PushCoalescer accumulates push-eligible notifications during a poll cycle and
// flushes them at cycle end as at most one push per (user, watch zone), plus
// one per-user "saved" bucket for zone-less notifications (GH#784). It owns the
// send-path collaborators the dispatchers used to call inline per notification:
// device lookup, the unread badge, the APNs sender, and zone-name resolution.
//
// PushCoalescer is driven by the single-goroutine poll handler loop (Reset at
// cycle start, Add during the authority walk, Flush at cycle end) and is not
// safe for concurrent use.
type PushCoalescer struct {
	devices deviceReader
	state   stateReader
	push    pushSender
	zones   zoneNameReader
	logger  *slog.Logger

	// items is userID -> bucketKey (watch zone id, or savedBucket) -> the
	// push-eligible notifications queued for that bucket this cycle.
	items map[string]map[string][]notifications.DigestNotification
}

// NewPushCoalescer wires the coalescer.
func NewPushCoalescer(devices deviceReader, state stateReader, push pushSender, zones zoneNameReader, logger *slog.Logger) *PushCoalescer {
	return &PushCoalescer{
		devices: devices,
		state:   state,
		push:    push,
		zones:   zones,
		logger:  logger,
		items:   make(map[string]map[string][]notifications.DigestNotification),
	}
}

// Add queues a push-eligible notification into its user's zone bucket (or the
// shared saved bucket for a zone-less notification). Called by the dispatchers
// in place of the old inline sendInstantPush — the coalescer does no gating,
// so only already push-eligible notifications must be handed to it.
func (c *PushCoalescer) Add(userID string, n notifications.DigestNotification) {
	bucket := savedBucket
	if n.WatchZoneID != nil {
		bucket = *n.WatchZoneID
	}
	if c.items[userID] == nil {
		c.items[userID] = make(map[string][]notifications.DigestNotification)
	}
	c.items[userID][bucket] = append(c.items[userID][bucket], n)
}

// Reset clears the accumulator. Called at the start of every poll cycle
// (before the authority loop) so a cycle only ever flushes its own pushes.
func (c *PushCoalescer) Reset() {
	c.items = make(map[string]map[string][]notifications.DigestNotification)
}

// Flush sends one push per accumulated (user, bucket) and then clears the
// accumulator. Every per-user and per-bucket failure is logged and swallowed —
// a push problem must never abort the flush or propagate to the poll cycle's
// result. Flush itself always returns nil; the error return exists so the
// polling handler's pushFlusher contract has somewhere to report a future
// failure without a signature change.
func (c *PushCoalescer) Flush(ctx context.Context) error {
	items := c.items
	c.items = make(map[string]map[string][]notifications.DigestNotification)

	for userID, buckets := range items {
		c.flushUser(ctx, userID, buckets)
	}
	return nil
}

// flushUser loads the user's devices and badge once, then sends exactly one
// push per bucket, accumulating any invalid tokens APNs reports across all of
// this user's sends and pruning the union once at the end.
func (c *PushCoalescer) flushUser(ctx context.Context, userID string, buckets map[string][]notifications.DigestNotification) {
	devices, err := c.devices.ListByUser(ctx, userID)
	if err != nil {
		c.logger.ErrorContext(ctx, "push coalescer: load devices failed", "user", userID, "error", err)
		return
	}
	if len(devices) == 0 {
		return
	}
	tokens := make([]string, 0, len(devices))
	for _, d := range devices {
		tokens = append(tokens, d.Token)
	}

	badge := c.unreadBadge(ctx, userID)
	zoneNames := c.zoneNames(ctx, userID)

	invalid := make(map[string]struct{})
	for bucketKey, queued := range buckets {
		payload, err := buildBucketPayload(bucketKey, queued, zoneNames, badge)
		if err != nil {
			c.logger.ErrorContext(ctx, "push coalescer: build payload failed", "user", userID, "bucket", bucketKey, "error", err)
			continue
		}
		invalidTokens, err := c.push.Send(ctx, tokens, payload)
		if err != nil {
			c.logger.ErrorContext(ctx, "push coalescer: send failed", "user", userID, "bucket", bucketKey, "error", err)
			continue
		}
		for _, token := range invalidTokens {
			invalid[token] = struct{}{}
		}
	}

	for token := range invalid {
		if err := c.devices.Delete(ctx, userID, token); err != nil {
			c.logger.WarnContext(ctx, "push coalescer: prune invalid token failed", "user", userID, "error", err)
		}
	}
}

// buildBucketPayload renders one bucket's push body: the existing rich
// single-app payload when exactly one notification queued (keeping its
// deep-link), otherwise the zone or saved summary.
func buildBucketPayload(bucketKey string, queued []notifications.DigestNotification, zoneNames map[string]string, badge int) (json.RawMessage, error) {
	if len(queued) == 1 {
		return buildAlertPayload(queued[0], badge)
	}
	if bucketKey == savedBucket {
		return buildSavedSummaryPayload(len(queued), badge)
	}
	return buildZoneSummaryPayload(len(queued), zoneNames[bucketKey], badge, bucketKey)
}

// unreadBadge computes the app-icon badge: the user's unread notifications
// (read_at IS NULL, ADR 0035). Unlike the old per-notification sendInstantPush,
// there is no +1 — every record in this cycle is already persisted by the time
// Flush runs, so the total unread count is already accurate.
func (c *PushCoalescer) unreadBadge(ctx context.Context, userID string) int {
	count, err := c.state.UnreadCount(ctx, userID)
	if err != nil {
		c.logger.ErrorContext(ctx, "push coalescer: unread count failed", "user", userID, "error", err)
		return 0
	}
	return count
}

// zoneNames resolves the user's watch zone id -> name map used to title a zone
// summary push, the same GetByUserID pattern the digest handler uses. A lookup
// failure logs and degrades to an empty map rather than aborting the flush.
func (c *PushCoalescer) zoneNames(ctx context.Context, userID string) map[string]string {
	zones, err := c.zones.GetByUserID(ctx, userID)
	if err != nil {
		c.logger.ErrorContext(ctx, "push coalescer: load zones failed", "user", userID, "error", err)
		return map[string]string{}
	}
	names := make(map[string]string, len(zones))
	for _, z := range zones {
		names[z.ID] = z.Name
	}
	return names
}
