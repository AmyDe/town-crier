// Package digest holds the Town Crier digest-generation worker modes: the weekly
// digest (WORKER_MODE=digest) and the hourly digest (WORKER_MODE=hourly-digest).
// Each cycle reads dispatched notifications from Cosmos, applies tier and
// per-user preference gating, groups the matching applications per watch zone,
// renders an email HTML body and (weekly, Pro tier only) an APNs push payload,
// hands them to the transport-only acsemail / apns senders, and records dedup
// state (the hourly cycle flips emailSent; the weekly push prunes invalid device
// tokens). It follows idiomatic Go: consumer-side interfaces, hand-written test
// fakes, business logic in the package.
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

// stateReader supplies the unread-count badge for the weekly push: the total
// unread tally (read_at IS NULL, ADR 0035). *notificationstate.PostgresStore
// satisfies it.
type stateReader interface {
	UnreadCount(ctx context.Context, userID string) (int, error)
}

// deviceReader lists a user's device tokens for a push and prunes the ones APNs
// reports permanently invalid. devicetokens.CosmosStore satisfies it.
type deviceReader interface {
	ListByUser(ctx context.Context, userID string) ([]devicetokens.DeviceRegistration, error)
	Delete(ctx context.Context, userID, token string) error
}

// pushDispatcher is the consumer-side platform-aware push contract; the concrete
// *notifydispatch.PlatformDispatcher satisfies it. It is declared locally (with
// the platform token split expressed in the signature) so the handler test can
// substitute a fake without importing notifydispatch.
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
	email         acsemail.EmailSender
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
	email acsemail.EmailSender,
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
			h.sendWeeklyPush(ctx, profile, len(notifs))
		}
		if wantsEmail {
			// The weekly cycle does not track emailSent (it re-derives the digest from
			// the look-back window each run), so a failed send is already logged inside
			// sendDigestEmail; we just move on to the next user.
			if err := h.sendDigestEmail(ctx, profile.UserID, *profile.Email, notifs); err != nil {
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

		// Only flip emailSent when the ACS send actually succeeded — one email
		// batches every included notification for this user, so a failed send must
		// leave the whole batch unmarked for the next cycle to retry. Marking on a
		// swallowed send error is silent data loss (tc-qvds).
		if err := h.sendDigestEmail(ctx, userID, *profile.Email, included); err != nil {
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

// sendDigestEmail collapses duplicate per-application records (see
// dedupByApplication — an application can legitimately have both a
// NewApplication and a DecisionUpdate record for the in-app feed, but the
// email must render each application once), groups the survivors by watch
// zone (with a saved-only section for zone-less notifications), renders the
// email body, and hands it to the transport-only email sender. Callers pass
// the full pre-dedup slice (RunHourly's MarkEmailSent loop relies on that: it
// walks its own pre-dedup `included` slice after this call returns, so the
// duplicate this function suppresses from the render is still marked sent and
// never resurfaces in a later cycle). It logs and returns any failure so the
// caller can decide whether to proceed: the hourly cycle must NOT flip
// emailSent when the send fails (otherwise the email is silently lost and
// never retried — tc-qvds), while the weekly cycle simply moves on to the next
// user. A failed email never aborts the rest of the cycle either way.
func (h *Handler) sendDigestEmail(ctx context.Context, userID, email string, notifs []notifications.DigestNotification) error {
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
		Subject:   buildDigestSubject(total),
		HTMLBody:  buildDigestHTML(sections, saved, total),
	}
	if err := h.email.Send(ctx, msg); err != nil {
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
