// Package digest runs the weekly (WORKER_MODE=digest) and hourly
// (WORKER_MODE=hourly-digest) digest cycles: it gates notifications by tier and
// preference, groups them per watch zone, renders the email and weekly push
// bodies, and records emailSent state.
package digest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/acsemail"
	"github.com/AmyDe/town-crier/api-go/internal/devicetokens"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// digestWindow is the 7-day look-back the weekly digest gathers notifications over.
const digestWindow = 7 * 24 * time.Hour

// email.kind values stamped on the "Email send" wrapper span, distinguishing the
// two digest cycles that share sendDigestEmail.
const (
	emailKindWeekly = "digest-weekly"
	emailKindHourly = "digest-hourly"
)

// emailSender is the instrumented email transport (*acsemail.InstrumentedSender).
type emailSender interface {
	Send(ctx context.Context, kind string, msg acsemail.Message) error
}

// profileReader selects users by digest day (weekly) and reads one user (hourly).
type profileReader interface {
	ByDigestDay(ctx context.Context, day time.Weekday) ([]*profiles.UserProfile, error)
	Get(ctx context.Context, userID string) (*profiles.UserProfile, error)
}

// notificationReader is the notifications store slice the digest worker uses.
type notificationReader interface {
	ByUserSince(ctx context.Context, userID string, since time.Time) ([]notifications.DigestNotification, error)
	UnsentEmailsByUser(ctx context.Context, userID string) ([]notifications.DigestNotification, error)
	UserIDsWithUnsentEmails(ctx context.Context) ([]string, error)
	MarkEmailSent(ctx context.Context, n notifications.DigestNotification) error
}

// zoneReader returns a user's watch zones for grouping and per-zone gating.
type zoneReader interface {
	GetByUserID(ctx context.Context, userID string) ([]watchzones.WatchZone, error)
}

// stateReader supplies the unread-count badge for the weekly push.
type stateReader interface {
	UnreadCount(ctx context.Context, userID string) (int, error)
}

// deviceReader lists a user's device tokens and prunes permanently invalid ones.
type deviceReader interface {
	ListByUser(ctx context.Context, userID string) ([]devicetokens.DeviceRegistration, error)
	Delete(ctx context.Context, userID, token string) error
}

// pushDispatcher is the platform-aware push sender
// (*notifydispatch.PlatformDispatcher).
type pushDispatcher interface {
	Send(ctx context.Context, iosTokens []string, iosPayload json.RawMessage, androidTokens []string, androidPayload json.RawMessage) ([]string, error)
}

// Handler runs the weekly and hourly digest cycles. It holds the stores and the
// transport-only senders, and renders the email/push bodies itself.
type Handler struct {
	profiles      profileReader
	notifications notificationReader
	zones         zoneReader
	state         stateReader
	devices       deviceReader
	email         emailSender
	dispatcher    pushDispatcher
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
	email emailSender,
	dispatcher pushDispatcher,
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
		dispatcher:    dispatcher,
		logger:        logger,
		now:           now,
	}
}

// RunWeekly generates the weekly digest for every user whose configured digest
// day is today. For each user it sends a digest email (when email is enabled and
// the user has an address) and a digest push (Pro tier only, when push is
// enabled).
func (h *Handler) RunWeekly(ctx context.Context) error {
	now := h.now().UTC()
	today := now.Weekday()
	since := now.Add(-digestWindow)

	users, err := h.profiles.ByDigestDay(ctx, today)
	if err != nil {
		return err
	}

	for _, profile := range users {
		// The weekly push is Pro-only; a lapsed paid tier (EffectiveTier) reads as
		// Free and gets the email but no push.
		wantsPush := profile.EffectiveTier(now).IsPaidPro() && profile.Preferences.PushEnabled
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
			// Dedup before counting so the push count matches the deduped email count.
			h.sendWeeklyPush(ctx, profile, len(dedupByApplication(notifs)))
		}
		if wantsEmail {
			// The weekly cycle does not track emailSent; sendDigestEmail logs a failed send.
			// IsPaid, not IsPaidPro, so a Personal subscriber never sees the free-tier notice.
			showFreeTierNotice := !profile.EffectiveTier(now).IsPaid()
			if err := h.sendDigestEmail(ctx, emailKindWeekly, profile.UserID, *profile.Email, notifs, showFreeTierNotice); err != nil {
				continue
			}
		}
	}
	return nil
}

// sendWeeklyPush builds and sends the weekly digest push across both platforms
// (APNs for iOS tokens, FCM for Android tokens), then prunes any device tokens
// either sender reports invalid. A user with no devices is a no-op. The badge is
// the total unread count (read_at IS NULL, ADR 0035), distinct from the digest
// application count; FCM carries no badge (Android badges are channel-driven).
func (h *Handler) sendWeeklyPush(ctx context.Context, profile *profiles.UserProfile, applicationCount int) {
	devices, err := h.devices.ListByUser(ctx, profile.UserID)
	if err != nil {
		h.logger.ErrorContext(ctx, "weekly digest: load devices failed", "user", profile.UserID, "error", err)
		return
	}
	if len(devices) == 0 {
		return
	}

	totalUnread, err := h.state.UnreadCount(ctx, profile.UserID)
	if err != nil {
		h.logger.ErrorContext(ctx, "weekly digest: unread count failed", "user", profile.UserID, "error", err)
		return
	}

	var iosTokens, androidTokens []string
	for _, d := range devices {
		if d.Platform == devicetokens.PlatformAndroid {
			androidTokens = append(androidTokens, d.Token)
			continue
		}
		iosTokens = append(iosTokens, d.Token)
	}

	var iosPayload, androidPayload json.RawMessage
	if len(iosTokens) > 0 {
		p, err := buildDigestPayload(applicationCount, totalUnread)
		if err != nil {
			h.logger.ErrorContext(ctx, "weekly digest: build apns payload failed", "user", profile.UserID, "error", err)
			return
		}
		iosPayload = p
	}
	if len(androidTokens) > 0 {
		p, err := buildDigestFCMPayload(applicationCount)
		if err != nil {
			h.logger.ErrorContext(ctx, "weekly digest: build fcm payload failed", "user", profile.UserID, "error", err)
			return
		}
		androidPayload = p
	}

	invalid, err := h.dispatcher.Send(ctx, iosTokens, iosPayload, androidTokens, androidPayload)
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
// up.
func (h *Handler) RunHourly(ctx context.Context) error {
	now := h.now()
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
		// is excluded even when email digests are enabled. A lapsed paid tier reads
		// as Free via EffectiveTier and is excluded too.
		if !profile.EffectiveTier(now).HasHourlyDigestEntitlement() {
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

		// Flip emailSent only after a successful send, so a failed send leaves the
		// whole batch unsent for the next cycle to retry.
		if err := h.sendDigestEmail(ctx, emailKindHourly, userID, *profile.Email, included, false); err != nil {
			continue
		}

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
// bookmark contract, which bypasses the per-zone gate).
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

// sendDigestEmail dedups notifs per application, groups them by watch zone,
// renders the email and sends it. Callers pass the pre-dedup slice, so the
// hourly cycle still marks a suppressed duplicate sent. It logs and returns any
// failure so the hourly cycle can skip marking emailSent.
func (h *Handler) sendDigestEmail(ctx context.Context, kind, userID, email string, notifs []notifications.DigestNotification, showFreeTierNotice bool) error {
	zones, err := h.zones.GetByUserID(ctx, userID)
	if err != nil {
		h.logger.ErrorContext(ctx, "digest email: load zones failed", "user", userID, "error", err)
		return fmt.Errorf("load zones for %s: %w", userID, err)
	}
	zoneName := make(map[string]string, len(zones))
	for _, z := range zones {
		zoneName[z.ID] = z.Name
	}

	deduped := dedupByApplication(notifs)
	sections, saved, total := groupByZone(deduped, zoneName)

	msg := acsemail.Message{
		Sender:    senderAddress,
		Recipient: email,
		Subject:   buildDigestSubject(total, sections, saved),
		HTMLBody:  buildDigestHTML(sections, saved, total, showFreeTierNotice),
	}
	if err := h.email.Send(ctx, kind, msg); err != nil {
		h.logger.ErrorContext(ctx, "digest email: send failed", "user", userID, "error", err)
		return fmt.Errorf("send digest email to %s: %w", userID, err)
	}
	return nil
}

// groupByZone partitions notifications into per-zone sections (preserving zone
// order by first appearance) plus a saved-only slice for zone-less notifications,
// returning the total count. An unknown zone id renders as "Unknown Zone"
// (fallback when the id is not in the zone-name map).
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
