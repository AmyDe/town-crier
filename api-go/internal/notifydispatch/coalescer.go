package notifydispatch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/devicetokens"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// deviceReader lists a user's device tokens for a push and prunes the ones APNs
// reports permanently invalid. *devicetokens.PostgresStore satisfies it.
type deviceReader interface {
	ListByUser(ctx context.Context, userID string) ([]devicetokens.DeviceRegistration, error)
	Delete(ctx context.Context, userID, token string) error
}

// collapsingSender is the APNs-side contract: pushSender plus a collapse ID.
// apns.PushSender satisfies it.
type collapsingSender interface {
	pushSender
	SendCollapsed(ctx context.Context, tokens []string, payload json.RawMessage, collapseID string) ([]string, error)
}

// stateReader supplies the unread-badge input: the total unread tally
// (read_at IS NULL, ADR 0035). *notificationstate.PostgresStore satisfies it.
type stateReader interface {
	UnreadCount(ctx context.Context, userID string) (int, error)
}

// pushSender is the consumer-side push contract; apns.PushSender (the real
// Client or the NoOpSender) satisfies it structurally.
type pushSender interface {
	Send(ctx context.Context, tokens []string, payload json.RawMessage) ([]string, error)
}

// pushMarker records a bucket's push lifecycle on the notification rows.
// *notifications.PostgresStore satisfies it.
type pushMarker interface {
	MarkPushAttempted(ctx context.Context, ids []string, at time.Time) error
	MarkPushStatus(ctx context.Context, ids []string, status notifications.PushStatus) error
}

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
// one per-user "saved" bucket for zone-less notifications. It owns device
// lookup, the unread badge, the platform dispatcher (APNs + FCM), and
// zone-name resolution.
//
// PushCoalescer is driven by the single-goroutine poll handler loop (Reset at
// cycle start, Add during the authority walk, Flush at cycle end) and is not
// safe for concurrent use.
type PushCoalescer struct {
	devices    deviceReader
	state      stateReader
	dispatcher *PlatformDispatcher
	zones      zoneNameReader
	marker     pushMarker
	now        func() time.Time
	logger     *slog.Logger

	// items is userID -> bucketKey (watch zone id, or savedBucket) -> the
	// push-eligible notifications queued for that bucket this cycle.
	items map[string]map[string][]notifications.DigestNotification
}

// NewPushCoalescer wires the coalescer.
func NewPushCoalescer(devices deviceReader, state stateReader, dispatcher *PlatformDispatcher, zones zoneNameReader, marker pushMarker, now func() time.Time, logger *slog.Logger) *PushCoalescer {
	return &PushCoalescer{
		devices:    devices,
		state:      state,
		dispatcher: dispatcher,
		zones:      zones,
		marker:     marker,
		now:        now,
		logger:     logger,
		items:      make(map[string]map[string][]notifications.DigestNotification),
	}
}

// Add queues a push-eligible notification into its user's zone bucket (or the
// shared saved bucket for a zone-less notification). The coalescer does no
// gating, so only already push-eligible notifications must be handed to it.
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
// push per bucket per platform (APNs for iOS tokens, FCM for Android tokens),
// accumulating any invalid tokens either sender reports across all of this
// user's sends and pruning the union once at the end. The per-platform payload
// is built only when the user has tokens on that platform.
func (c *PushCoalescer) flushUser(ctx context.Context, userID string, buckets map[string][]notifications.DigestNotification) {
	devices, err := c.devices.ListByUser(ctx, userID)
	if err != nil {
		c.logger.ErrorContext(ctx, "push coalescer: load devices failed", "user", userID, "error", err)
		return
	}
	if len(devices) == 0 {
		for bucketKey, queued := range buckets {
			c.mark(ctx, userID, bucketKey, notificationIDs(queued), notifications.PushStatusSkipped)
		}
		return
	}
	iosTokens, androidTokens := groupTokensByPlatform(devices)

	badge := c.unreadBadge(ctx, userID)
	zoneNames := c.zoneNames(ctx, userID)

	invalid := make(map[string]struct{})
	for bucketKey, queued := range buckets {
		ids := notificationIDs(queued)
		collapseID := pushCollapseID(ids)
		var iosPayload, androidPayload json.RawMessage
		if len(iosTokens) > 0 {
			p, err := buildBucketPayload(bucketKey, queued, zoneNames, badge)
			if err != nil {
				c.logger.ErrorContext(ctx, "push coalescer: build apns payload failed", "user", userID, "bucket", bucketKey, "error", err)
				continue
			}
			iosPayload = p
		}
		if len(androidTokens) > 0 {
			p, err := buildBucketFCMPayload(bucketKey, queued, zoneNames, collapseID)
			if err != nil {
				c.logger.ErrorContext(ctx, "push coalescer: build fcm payload failed", "user", userID, "bucket", bucketKey, "error", err)
				continue
			}
			androidPayload = p
		}

		if err := c.marker.MarkPushAttempted(ctx, ids, c.now()); err != nil {
			c.logMarkFailed(ctx, userID, bucketKey, err)
		}
		invalidTokens, err := c.dispatcher.SendCollapsed(ctx, collapseID, iosTokens, iosPayload, androidTokens, androidPayload)
		if err != nil {
			c.logger.ErrorContext(ctx, "push coalescer: send failed", "user", userID, "bucket", bucketKey, "error", err)
			continue
		}
		c.mark(ctx, userID, bucketKey, ids, notifications.PushStatusSent)
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

func (c *PushCoalescer) mark(ctx context.Context, userID, bucketKey string, ids []string, status notifications.PushStatus) {
	if err := c.marker.MarkPushStatus(ctx, ids, status); err != nil {
		c.logMarkFailed(ctx, userID, bucketKey, err)
	}
}

func (c *PushCoalescer) logMarkFailed(ctx context.Context, userID, bucketKey string, err error) {
	c.logger.WarnContext(ctx, "push.mark_failed", "user", userID, "bucket", bucketKey, "error", err)
}

func notificationIDs(queued []notifications.DigestNotification) []string {
	ids := make([]string, len(queued))
	for i, n := range queued {
		ids[i] = n.ID
	}
	return ids
}

// pushCollapseID returns the APNs collapse ID (and FCM tag) for a bucket's
// push: the notification ID for a single push, otherwise "s-" plus the first 40
// hex characters of SHA-256 over the sorted IDs joined with commas. It is
// stable for a given set of IDs regardless of order, so a resend of the same
// set replaces the first copy; both forms stay under APNs' 64-byte limit.
func pushCollapseID(ids []string) string {
	if len(ids) == 1 {
		return ids[0]
	}
	sorted := append([]string(nil), ids...)
	sort.Strings(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, ",")))
	return "s-" + hex.EncodeToString(sum[:])[:40]
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
// (read_at IS NULL, ADR 0035). There is no +1: every record in this cycle is
// already persisted by the time Flush runs.
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
