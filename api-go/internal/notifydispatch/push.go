package notifydispatch

import (
	"context"
	"log/slog"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/notifications"
)

// instantPushDeps bundles the collaborators a single-notification push needs, so
// the enqueuer and decision dispatcher share one push path rather than each
// re-implementing device load + badge + send + prune.
type instantPushDeps struct {
	devices deviceReader
	state   stateReader
	push    pushSender
	logger  *slog.Logger
}

// sendInstantPush loads the user's devices, builds the alert payload, sends it,
// prunes any tokens APNs reports invalid, and reports whether a push was actually
// sent. A user with no devices is a no-op (false). Every failure is logged and
// swallowed — a push failure must not abort the record write, mirroring .NET.
func sendInstantPush(ctx context.Context, deps instantPushDeps, userID string, n notifications.DigestNotification) bool {
	devices, err := deps.devices.ListByUser(ctx, userID)
	if err != nil {
		deps.logger.ErrorContext(ctx, "instant push: load devices failed", "user", userID, "error", err)
		return false
	}
	if len(devices) == 0 {
		return false
	}

	badge := unreadBadge(ctx, deps.state, deps.logger, userID)
	payload, err := buildAlertPayload(n, badge)
	if err != nil {
		deps.logger.ErrorContext(ctx, "instant push: build payload failed", "user", userID, "error", err)
		return false
	}

	tokens := make([]string, 0, len(devices))
	for _, d := range devices {
		tokens = append(tokens, d.Token)
	}

	invalid, err := deps.push.Send(ctx, tokens, payload)
	if err != nil {
		deps.logger.ErrorContext(ctx, "instant push: send failed", "user", userID, "error", err)
		return false
	}
	for _, token := range invalid {
		if err := deps.devices.Delete(ctx, userID, token); err != nil {
			deps.logger.WarnContext(ctx, "instant push: prune invalid token failed", "user", userID, "error", err)
		}
	}
	return true
}

// unreadBadge computes the app-icon badge: notifications created strictly after
// the read watermark plus 1 for the just-created notification (not yet persisted
// but unread by construction). A first-touch user (no watermark) starts at the
// Unix epoch so everything counts, matching .NET's MinValue + 1.
func unreadBadge(ctx context.Context, state stateReader, logger *slog.Logger, userID string) int {
	lastReadAt := time.Unix(0, 0).UTC()
	st, err := state.Get(ctx, userID)
	if err != nil {
		logger.ErrorContext(ctx, "instant push: load notification state failed", "user", userID, "error", err)
		return 1
	}
	if st != nil {
		lastReadAt = st.LastReadAt
	}
	count, err := state.UnreadCount(ctx, userID, lastReadAt)
	if err != nil {
		logger.ErrorContext(ctx, "instant push: unread count failed", "user", userID, "error", err)
		return 1
	}
	return count + 1
}
