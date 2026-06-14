// Package digest holds the Town Crier digest-generation worker modes: the weekly
// digest (WORKER_MODE=digest) and the hourly digest (WORKER_MODE=hourly-digest).
// Each cycle reads dispatched notifications from Cosmos, applies tier and
// per-user preference gating, groups the matching applications per watch zone,
// renders an email HTML body and (weekly, Pro tier only) an APNs push payload,
// hands them to the transport-only acsemail / apns senders, and records dedup
// state (the hourly cycle flips emailSent; the weekly push prunes invalid device
// tokens). It ports the .NET GenerateWeeklyDigests / GenerateHourlyDigests
// command handlers (epic tc-wad3) following idiomatic Go: consumer-side
// interfaces, hand-written test fakes, business logic in the package.
package digest

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/acsemail"
	"github.com/AmyDe/town-crier/api-go/internal/devicetokens"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/notificationstate"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// digestWindow is the look-back the weekly digest gathers notifications over,
// matching .NET's now.AddDays(-7).
const digestWindow = 7 * 24 * time.Hour

// profileReader is the consumer-side slice of the profile stores the digest
// worker needs: the weekly cycle selects by digest day (cross-partition) and the
// hourly cycle point-reads each candidate user. profiles.AdminStore satisfies
// ByDigestDay; profiles.CosmosStore satisfies Get.
type profileReader interface {
	ByDigestDay(ctx context.Context, day time.Weekday) ([]*profiles.UserProfile, error)
	Get(ctx context.Context, userID string) (*profiles.UserProfile, error)
}

// notificationReader is the consumer-side slice of the Notifications store the
// digest worker reads and writes. notifications.DigestStore satisfies it.
type notificationReader interface {
	ByUserSince(ctx context.Context, userID string, since time.Time) ([]notifications.DigestNotification, error)
	UnsentEmailsByUser(ctx context.Context, userID string) ([]notifications.DigestNotification, error)
	UserIDsWithUnsentEmails(ctx context.Context) ([]string, error)
	MarkEmailSent(ctx context.Context, n notifications.DigestNotification) error
}

// zoneReader returns a user's watch zones for grouping and per-zone gating.
// watchzones.CosmosStore satisfies it.
type zoneReader interface {
	GetByUserID(ctx context.Context, userID string) ([]watchzones.WatchZone, error)
}

// stateReader supplies the unread-count badge inputs for the weekly push:
// the watermark and the strictly-after-watermark count. notificationstate.CosmosStore
// satisfies it.
type stateReader interface {
	Get(ctx context.Context, userID string) (*notificationstate.State, error)
	UnreadCount(ctx context.Context, userID string, lastReadAt time.Time) (int, error)
}

// deviceReader lists a user's device tokens for a push and prunes the ones APNs
// reports permanently invalid. devicetokens.CosmosStore satisfies it.
type deviceReader interface {
	ListByUser(ctx context.Context, userID string) ([]devicetokens.DeviceRegistration, error)
	Delete(ctx context.Context, userID, token string) error
}

// pushSender is the consumer-side push contract; apns.PushSender (the real Client
// or the NoOpSender) satisfies it. It is declared locally so the handler test can
// substitute a spy without importing apns.
type pushSender interface {
	Send(ctx context.Context, tokens []string, payload json.RawMessage) ([]string, error)
}

// Handler runs the weekly and hourly digest cycles. It holds the stores and the
// transport-only senders, and renders the email/push bodies itself.
type Handler struct {
	profiles      profileReader
	notifications notificationReader
	zones         zoneReader
	state         stateReader
	devices       deviceReader
	email         acsemail.EmailSender
	push          pushSender
	logger        *slog.Logger
	now           func() time.Time
}

// NewHandler wires the digest handler. now is injected so tests can pin the
// current day (which drives the weekly digest-day selection); production passes
// time.Now.
func NewHandler(
	profiles profileReader,
	notifications notificationReader,
	zones zoneReader,
	state stateReader,
	devices deviceReader,
	email acsemail.EmailSender,
	push pushSender,
	logger *slog.Logger,
	now func() time.Time,
) *Handler {
	return &Handler{
		profiles:      profiles,
		notifications: notifications,
		zones:         zones,
		state:         state,
		devices:       devices,
		email:         email,
		push:          push,
		logger:        logger,
		now:           now,
	}
}

// RunWeekly generates the weekly digest for every user whose configured digest
// day is today. For each user it sends a digest email (when email is enabled and
// the user has an address) and a digest push (Pro tier only, when push is
// enabled). It ports .NET GenerateWeeklyDigestsCommandHandler.
func (h *Handler) RunWeekly(ctx context.Context) error {
	now := h.now().UTC()
	today := now.Weekday()
	since := now.Add(-digestWindow)

	users, err := h.profiles.ByDigestDay(ctx, today)
	if err != nil {
		return err
	}

	for _, profile := range users {
		wantsPush := profile.Tier.IsPaidPro() && profile.Preferences.PushEnabled
		wantsEmail := profile.Preferences.EmailDigestEnabled && profile.Email != nil && *profile.Email != ""
		if !wantsPush && !wantsEmail {
			continue
		}

		notifs, err := h.notifications.ByUserSince(ctx, profile.UserID, since)
		if err != nil {
			h.logger.ErrorContext(ctx, "weekly digest: load notifications failed", "user", profile.UserID, "error", err)
			continue
		}
		if len(notifs) == 0 {
			continue
		}

		if wantsPush {
			h.sendWeeklyPush(ctx, profile, len(notifs))
		}
		if wantsEmail {
			h.sendDigestEmail(ctx, profile.UserID, *profile.Email, notifs)
		}
	}
	return nil
}

// sendWeeklyPush builds and sends the weekly digest push, then prunes any device
// tokens APNs reports invalid. A user with no devices is a no-op. The badge is
// the total unread count derived from the watermark (first-touch users start at
// the zero instant), distinct from the digest application count.
func (h *Handler) sendWeeklyPush(ctx context.Context, profile *profiles.UserProfile, applicationCount int) {
	devices, err := h.devices.ListByUser(ctx, profile.UserID)
	if err != nil {
		h.logger.ErrorContext(ctx, "weekly digest: load devices failed", "user", profile.UserID, "error", err)
		return
	}
	if len(devices) == 0 {
		return
	}

	lastReadAt := time.Unix(0, 0).UTC()
	st, err := h.state.Get(ctx, profile.UserID)
	if err != nil {
		h.logger.ErrorContext(ctx, "weekly digest: load notification state failed", "user", profile.UserID, "error", err)
		return
	}
	if st != nil {
		lastReadAt = st.LastReadAt
	}
	totalUnread, err := h.state.UnreadCount(ctx, profile.UserID, lastReadAt)
	if err != nil {
		h.logger.ErrorContext(ctx, "weekly digest: unread count failed", "user", profile.UserID, "error", err)
		return
	}

	payload, err := buildDigestPayload(applicationCount, totalUnread)
	if err != nil {
		h.logger.ErrorContext(ctx, "weekly digest: build payload failed", "user", profile.UserID, "error", err)
		return
	}

	tokens := make([]string, 0, len(devices))
	for _, d := range devices {
		tokens = append(tokens, d.Token)
	}

	invalid, err := h.push.Send(ctx, tokens, payload)
	if err != nil {
		h.logger.ErrorContext(ctx, "weekly digest: push send failed", "user", profile.UserID, "error", err)
		return
	}
	for _, token := range invalid {
		if err := h.devices.Delete(ctx, profile.UserID, token); err != nil {
			h.logger.WarnContext(ctx, "weekly digest: prune invalid token failed", "user", profile.UserID, "error", err)
		}
	}
}

// RunHourly generates the hourly digest email for every paid user with unsent-email
// notifications, honouring per-zone instant-email gating. It marks the included
// notifications email-sent so the next cycle excludes them; excluded (per-zone
// disabled) notifications are left unsent so the weekly digest can still pick them
// up. It ports .NET GenerateHourlyDigestsCommandHandler.
func (h *Handler) RunHourly(ctx context.Context) error {
	userIDs, err := h.notifications.UserIDsWithUnsentEmails(ctx)
	if err != nil {
		return err
	}

	for _, userID := range userIDs {
		profile, err := h.profiles.Get(ctx, userID)
		if err != nil {
			if errors.Is(err, profiles.ErrNotFound) {
				continue
			}
			h.logger.ErrorContext(ctx, "hourly digest: load profile failed", "user", userID, "error", err)
			continue
		}
		// Hourly digest emails are a paid entitlement (server-enforced) — Free tier
		// is excluded even when email digests are enabled.
		if !profile.Tier.HasHourlyDigestEntitlement() {
			continue
		}
		if profile.Email == nil || *profile.Email == "" {
			continue
		}
		if !profile.Preferences.EmailDigestEnabled {
			continue
		}

		notifs, err := h.notifications.UnsentEmailsByUser(ctx, userID)
		if err != nil {
			h.logger.ErrorContext(ctx, "hourly digest: load unsent emails failed", "user", userID, "error", err)
			continue
		}
		if len(notifs) == 0 {
			continue
		}

		zones, err := h.zones.GetByUserID(ctx, userID)
		if err != nil {
			h.logger.ErrorContext(ctx, "hourly digest: load zones failed", "user", userID, "error", err)
			continue
		}

		included := filterByInstantGate(notifs, zones)
		if len(included) == 0 {
			continue
		}

		h.sendDigestEmail(ctx, userID, *profile.Email, included)

		for _, n := range included {
			if err := h.notifications.MarkEmailSent(ctx, markSent(n)); err != nil {
				h.logger.ErrorContext(ctx, "hourly digest: mark email sent failed", "user", userID, "notification", n.ID, "error", err)
			}
		}
	}
	return nil
}

// filterByInstantGate keeps notifications whose watch zone has instant email
// enabled, plus all saved-only notifications (no zone — driven by the saved
// bookmark contract, which bypasses the per-zone gate). Mirrors .NET's
// instantEnabledZones filter.
func filterByInstantGate(notifs []notifications.DigestNotification, zones []watchzones.WatchZone) []notifications.DigestNotification {
	instantEnabled := make(map[string]struct{}, len(zones))
	for _, z := range zones {
		if z.EmailInstantEnabled {
			instantEnabled[z.ID] = struct{}{}
		}
	}
	included := make([]notifications.DigestNotification, 0, len(notifs))
	for _, n := range notifs {
		if n.WatchZoneID == nil {
			included = append(included, n)
			continue
		}
		if _, ok := instantEnabled[*n.WatchZoneID]; ok {
			included = append(included, n)
		}
	}
	return included
}

// markSent returns a copy of n with EmailSent set, ready to upsert.
func markSent(n notifications.DigestNotification) notifications.DigestNotification {
	n.MarkEmailSent()
	return n
}

// sendDigestEmail groups the notifications by watch zone (with a saved-only
// section for zone-less notifications), renders the email body, and hands it to
// the transport-only email sender. A send failure is logged, not propagated — one
// failed email must not abort the rest of the cycle, mirroring .NET's
// catch-and-continue in AcsEmailSender.
func (h *Handler) sendDigestEmail(ctx context.Context, userID, email string, notifs []notifications.DigestNotification) {
	zones, err := h.zones.GetByUserID(ctx, userID)
	if err != nil {
		h.logger.ErrorContext(ctx, "digest email: load zones failed", "user", userID, "error", err)
		return
	}
	zoneName := make(map[string]string, len(zones))
	for _, z := range zones {
		zoneName[z.ID] = z.Name
	}

	sections, saved, total := groupByZone(notifs, zoneName)

	msg := acsemail.Message{
		Sender:    senderAddress,
		Recipient: email,
		Subject:   buildDigestSubject(total),
		HTMLBody:  buildDigestHTML(sections, saved, total),
	}
	if err := h.email.Send(ctx, msg); err != nil {
		h.logger.ErrorContext(ctx, "digest email: send failed", "user", userID, "error", err)
	}
}

// groupByZone partitions notifications into per-zone sections (preserving zone
// order by first appearance) plus a saved-only slice for zone-less notifications,
// returning the total count. An unknown zone id renders as "Unknown Zone",
// mirroring .NET's zoneLookup.GetValueOrDefault fallback.
func groupByZone(notifs []notifications.DigestNotification, zoneName map[string]string) (sections []watchZoneDigest, saved []notifications.DigestNotification, total int) {
	index := map[string]int{}
	for _, n := range notifs {
		total++
		if n.WatchZoneID == nil {
			saved = append(saved, n)
			continue
		}
		zoneID := *n.WatchZoneID
		pos, ok := index[zoneID]
		if !ok {
			name := "Unknown Zone"
			if display, found := zoneName[zoneID]; found {
				name = display
			}
			sections = append(sections, watchZoneDigest{name: name})
			pos = len(sections) - 1
			index[zoneID] = pos
		}
		sections[pos].notifications = append(sections[pos].notifications, n)
	}
	return sections, saved, total
}
