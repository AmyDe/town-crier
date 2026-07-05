package digest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/acsemail"
	"github.com/AmyDe/town-crier/api-go/internal/devicetokens"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

const (
	testUser  = "user-1"
	testEmail = "u1@example.com"
)

// ---- hand-written fakes ----

type fakeProfiles struct {
	byDay    map[time.Weekday][]*profiles.UserProfile
	byID     map[string]*profiles.UserProfile
	byDayErr error
}

func (f *fakeProfiles) ByDigestDay(_ context.Context, day time.Weekday) ([]*profiles.UserProfile, error) {
	if f.byDayErr != nil {
		return nil, f.byDayErr
	}
	return f.byDay[day], nil
}

func (f *fakeProfiles) Get(_ context.Context, userID string) (*profiles.UserProfile, error) {
	p, ok := f.byID[userID]
	if !ok {
		return nil, profiles.ErrNotFound
	}
	return p, nil
}

type fakeNotifications struct {
	sinceByUser   map[string][]notifications.DigestNotification
	unsentByUser  map[string][]notifications.DigestNotification
	unsentUserIDs []string
	marked        []string // notification IDs marked email-sent
}

func (f *fakeNotifications) ByUserSince(_ context.Context, userID string, _ time.Time) ([]notifications.DigestNotification, error) {
	return f.sinceByUser[userID], nil
}

func (f *fakeNotifications) UnsentEmailsByUser(_ context.Context, userID string) ([]notifications.DigestNotification, error) {
	return f.unsentByUser[userID], nil
}

func (f *fakeNotifications) UserIDsWithUnsentEmails(_ context.Context) ([]string, error) {
	return f.unsentUserIDs, nil
}

func (f *fakeNotifications) MarkEmailSent(_ context.Context, n notifications.DigestNotification) error {
	f.marked = append(f.marked, n.ID)
	return nil
}

type fakeZones struct {
	byUser map[string][]watchzones.WatchZone
}

func (f *fakeZones) GetByUserID(_ context.Context, userID string) ([]watchzones.WatchZone, error) {
	return f.byUser[userID], nil
}

type fakeState struct {
	unread map[string]int
}

func (f *fakeState) UnreadCount(_ context.Context, userID string) (int, error) {
	return f.unread[userID], nil
}

type fakeDevices struct {
	byUser  map[string][]devicetokens.DeviceRegistration
	deleted []string // tokens deleted
}

func (f *fakeDevices) ListByUser(_ context.Context, userID string) ([]devicetokens.DeviceRegistration, error) {
	return f.byUser[userID], nil
}

func (f *fakeDevices) Delete(_ context.Context, _ string, token string) error {
	f.deleted = append(f.deleted, token)
	return nil
}

type spyEmail struct {
	sent    []acsemail.Message
	sendErr error // returned (after recording the attempt) when set
}

func (s *spyEmail) Send(_ context.Context, msg acsemail.Message) error {
	s.sent = append(s.sent, msg)
	return s.sendErr
}

type spyPush struct {
	calls   int
	tokens  []string
	payload json.RawMessage
	invalid []string // tokens to report invalid on the next Send
}

func (s *spyPush) Send(_ context.Context, tokens []string, payload json.RawMessage) ([]string, error) {
	s.calls++
	s.tokens = tokens
	s.payload = payload
	return s.invalid, nil
}

// fakeDispatcher mirrors the real *notifydispatch.PlatformDispatcher's token
// split: it routes the iOS payload to ios and the Android payload to android,
// unioning any invalid tokens. It lets the digest handler test assert
// platform-aware delivery without importing notifydispatch — and lets the
// pre-existing iOS-only weekly tests keep inspecting the ios spy unchanged.
type fakeDispatcher struct {
	ios     *spyPush
	android *spyPush
}

func (d *fakeDispatcher) Send(ctx context.Context, iosTokens []string, iosPayload json.RawMessage, androidTokens []string, androidPayload json.RawMessage) ([]string, error) {
	var invalid []string
	if len(iosTokens) > 0 {
		inv, _ := d.ios.Send(ctx, iosTokens, iosPayload)
		invalid = append(invalid, inv...)
	}
	if len(androidTokens) > 0 {
		inv, _ := d.android.Send(ctx, androidTokens, androidPayload)
		invalid = append(invalid, inv...)
	}
	return invalid, nil
}

// ---- helpers ----

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
}

func mkProfile(t *testing.T, tier profiles.SubscriptionTier, prefs profiles.NotificationPreferences) *profiles.UserProfile {
	t.Helper()
	p, err := profiles.NewProfile(testUser, testEmail, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}
	p.Tier = tier
	p.Preferences = prefs
	return p
}

func mkZone(t *testing.T, id, name string, emailInstant bool) watchzones.WatchZone {
	t.Helper()
	z, err := watchzones.NewWatchZone(id, testUser, name, 51.5, -0.1, 500, 1, time.Now(), true, emailInstant)
	if err != nil {
		t.Fatalf("NewWatchZone: %v", err)
	}
	return z
}

func zoneNotif(uid, zoneID string) notifications.DigestNotification {
	z := zoneID
	return notifications.DigestNotification{
		ID:                     "n-" + uid,
		UserID:                 testUser,
		ApplicationUID:         uid,
		ApplicationName:        uid,
		WatchZoneID:            &z,
		ApplicationAddress:     "addr " + uid,
		ApplicationDescription: "desc",
		EventType:              notifications.EventNewApplication,
		AuthorityID:            1,
		CreatedAt:              time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	}
}

// newHandler wires a handler whose weekly push routes iOS tokens to the given
// spy (the platform the pre-existing weekly tests exercise). The Android sender
// is a throwaway spy those tests never reach; tests needing the FCM path build
// their own fakeDispatcher via newHandlerWithDispatcher.
func newHandler(p *fakeProfiles, n *fakeNotifications, z *fakeZones, st *fakeState, d *fakeDevices, email acsemail.EmailSender, push *spyPush) *Handler {
	return newHandlerWithDispatcher(p, n, z, st, d, email, &fakeDispatcher{ios: push, android: &spyPush{}})
}

func newHandlerWithDispatcher(p *fakeProfiles, n *fakeNotifications, z *fakeZones, st *fakeState, d *fakeDevices, email acsemail.EmailSender, dispatcher pushDispatcher) *Handler {
	return NewHandler(p, n, z, st, d, email, dispatcher, testLogger(), func() time.Time {
		// Pin "now" to a Wednesday so weekly tests select the Wednesday digest day.
		return time.Date(2026, 2, 4, 9, 0, 0, 0, time.UTC) // 2026-02-04 is a Wednesday
	})
}

// ---- weekly tests ----

func TestRunWeekly_EmailGroupsByZoneAndSavedSection(t *testing.T) {
	t.Parallel()
	prefs := profiles.DefaultPreferences()
	prefs.EmailDigestEnabled = true
	prefs.PushEnabled = false
	p := mkProfile(t, profiles.TierFree, prefs)

	zoneN := zoneNotif("uid-A", "zone-1")
	savedN := zoneNotif("uid-B", "")
	savedN.WatchZoneID = nil // saved-only (no zone)

	fp := &fakeProfiles{byDay: map[time.Weekday][]*profiles.UserProfile{time.Wednesday: {p}}}
	fn := &fakeNotifications{sinceByUser: map[string][]notifications.DigestNotification{
		"user-1": {zoneN, savedN},
	}}
	fz := &fakeZones{byUser: map[string][]watchzones.WatchZone{
		"user-1": {mkZone(t, "zone-1", "Home", true)},
	}}
	email := &spyEmail{}
	push := &spyPush{}
	h := newHandler(fp, fn, fz, &fakeState{}, &fakeDevices{}, email, push)

	if err := h.RunWeekly(context.Background()); err != nil {
		t.Fatalf("RunWeekly: %v", err)
	}

	if len(email.sent) != 1 {
		t.Fatalf("emails sent: got %d, want 1", len(email.sent))
	}
	msg := email.sent[0]
	if msg.Recipient != "u1@example.com" || msg.Sender != senderAddress {
		t.Errorf("recipient/sender: got %q / %q", msg.Recipient, msg.Sender)
	}
	if !contains(msg.HTMLBody, "Home") || !contains(msg.HTMLBody, "Saved Applications") {
		t.Errorf("body missing zone or saved section:\n%s", msg.HTMLBody)
	}
	if !contains(msg.HTMLBody, "addr uid-A") || !contains(msg.HTMLBody, "addr uid-B") {
		t.Errorf("body missing notification addresses:\n%s", msg.HTMLBody)
	}
	// Email-only profile: no push.
	if push.calls != 0 {
		t.Errorf("push calls: got %d, want 0 (email-only profile)", push.calls)
	}
}

func TestRunWeekly_FreeTierGetsEmailButNoPush(t *testing.T) {
	t.Parallel()
	// Free tier: weekly digest EMAIL is allowed; the digest PUSH is Pro-only.
	prefs := profiles.DefaultPreferences()
	prefs.EmailDigestEnabled = true
	prefs.PushEnabled = true // push is on, but tier is Free -> no push
	p := mkProfile(t, profiles.TierFree, prefs)

	fp := &fakeProfiles{byDay: map[time.Weekday][]*profiles.UserProfile{time.Wednesday: {p}}}
	fn := &fakeNotifications{sinceByUser: map[string][]notifications.DigestNotification{
		"user-1": {zoneNotif("uid-A", "zone-1")},
	}}
	fz := &fakeZones{byUser: map[string][]watchzones.WatchZone{
		"user-1": {mkZone(t, "zone-1", "Home", true)},
	}}
	devices := &fakeDevices{byUser: map[string][]devicetokens.DeviceRegistration{
		"user-1": {{UserID: "user-1", Token: "tok-1", Platform: devicetokens.PlatformIos}},
	}}
	email := &spyEmail{}
	push := &spyPush{}
	h := newHandler(fp, fn, fz, &fakeState{}, devices, email, push)

	if err := h.RunWeekly(context.Background()); err != nil {
		t.Fatalf("RunWeekly: %v", err)
	}
	if len(email.sent) != 1 {
		t.Errorf("free tier should still get the weekly email: got %d", len(email.sent))
	}
	if push.calls != 0 {
		t.Errorf("free tier must NOT get a digest push: got %d push calls", push.calls)
	}
}

func TestRunWeekly_ProTierGetsPushWithBadgeAndPrunesInvalidTokens(t *testing.T) {
	t.Parallel()
	prefs := profiles.DefaultPreferences()
	prefs.EmailDigestEnabled = false
	prefs.PushEnabled = true
	p := mkProfile(t, profiles.TierPro, prefs)

	fp := &fakeProfiles{byDay: map[time.Weekday][]*profiles.UserProfile{time.Wednesday: {p}}}
	fn := &fakeNotifications{sinceByUser: map[string][]notifications.DigestNotification{
		"user-1": {zoneNotif("uid-A", "zone-1"), zoneNotif("uid-B", "zone-1")},
	}}
	state := &fakeState{
		unread: map[string]int{"user-1": 5},
	}
	devices := &fakeDevices{byUser: map[string][]devicetokens.DeviceRegistration{
		"user-1": {
			{UserID: "user-1", Token: "good-token", Platform: devicetokens.PlatformIos},
			{UserID: "user-1", Token: "dead-token", Platform: devicetokens.PlatformIos},
		},
	}}
	email := &spyEmail{}
	push := &spyPush{invalid: []string{"dead-token"}}
	h := newHandler(fp, fn, &fakeZones{}, state, devices, email, push)

	if err := h.RunWeekly(context.Background()); err != nil {
		t.Fatalf("RunWeekly: %v", err)
	}
	if push.calls != 1 {
		t.Fatalf("push calls: got %d, want 1", push.calls)
	}
	if len(push.tokens) != 2 {
		t.Errorf("push tokens: got %v, want 2", push.tokens)
	}
	// Badge = total unread count (5), body = 2 applications this week.
	var parsed struct {
		Aps struct {
			Alert struct {
				Body string `json:"body"`
			} `json:"alert"`
			Badge int `json:"badge"`
		} `json:"aps"`
	}
	if err := json.Unmarshal(push.payload, &parsed); err != nil {
		t.Fatalf("payload unmarshal: %v", err)
	}
	if parsed.Aps.Badge != 5 {
		t.Errorf("badge: got %d, want 5 (total unread)", parsed.Aps.Badge)
	}
	if parsed.Aps.Alert.Body != "2 new applications this week" {
		t.Errorf("body: got %q", parsed.Aps.Alert.Body)
	}
	// Invalid token pruned.
	if len(devices.deleted) != 1 || devices.deleted[0] != "dead-token" {
		t.Errorf("pruned tokens: got %v, want [dead-token]", devices.deleted)
	}
}

func TestRunWeekly_ExpiredProTierGetsNoPush(t *testing.T) {
	t.Parallel()
	// A Pro tier whose subscription has lapsed (past expiry, no grace) reads as
	// Free: the weekly digest PUSH is Pro-only, so no push fires even though push
	// is enabled and there are notifications.
	prefs := profiles.DefaultPreferences()
	prefs.EmailDigestEnabled = false
	prefs.PushEnabled = true
	p := mkProfile(t, profiles.TierPro, prefs)
	past := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) // before the harness clock (2026-02-04)
	p.SubscriptionExpiry = &past

	fp := &fakeProfiles{byDay: map[time.Weekday][]*profiles.UserProfile{time.Wednesday: {p}}}
	fn := &fakeNotifications{sinceByUser: map[string][]notifications.DigestNotification{
		"user-1": {zoneNotif("uid-A", "zone-1")},
	}}
	devices := &fakeDevices{byUser: map[string][]devicetokens.DeviceRegistration{
		"user-1": {{UserID: "user-1", Token: "tok-1", Platform: devicetokens.PlatformIos}},
	}}
	email := &spyEmail{}
	push := &spyPush{}
	h := newHandler(fp, fn, &fakeZones{}, &fakeState{}, devices, email, push)

	if err := h.RunWeekly(context.Background()); err != nil {
		t.Fatalf("RunWeekly: %v", err)
	}
	if push.calls != 0 {
		t.Errorf("lapsed Pro tier must NOT get a digest push: got %d push calls", push.calls)
	}
}

func TestRunWeekly_SkipsUsersWithNoNotifications(t *testing.T) {
	t.Parallel()
	prefs := profiles.DefaultPreferences()
	p := mkProfile(t, profiles.TierPro, prefs)

	fp := &fakeProfiles{byDay: map[time.Weekday][]*profiles.UserProfile{time.Wednesday: {p}}}
	fn := &fakeNotifications{} // no notifications for anyone
	email := &spyEmail{}
	push := &spyPush{}
	h := newHandler(fp, fn, &fakeZones{}, &fakeState{}, &fakeDevices{}, email, push)

	if err := h.RunWeekly(context.Background()); err != nil {
		t.Fatalf("RunWeekly: %v", err)
	}
	if len(email.sent) != 0 || push.calls != 0 {
		t.Errorf("no-notification user should be skipped: emails=%d pushes=%d", len(email.sent), push.calls)
	}
}

// ---- hourly tests ----

func TestRunHourly_PaidTierSendsAndMarksEmailSent(t *testing.T) {
	t.Parallel()
	prefs := profiles.DefaultPreferences()
	prefs.EmailDigestEnabled = true
	p := mkProfile(t, profiles.TierPro, prefs)

	n1 := zoneNotif("uid-A", "zone-1")
	n2 := zoneNotif("uid-B", "") // saved-only bypasses per-zone gate
	n2.WatchZoneID = nil

	fp := &fakeProfiles{byID: map[string]*profiles.UserProfile{"user-1": p}}
	fn := &fakeNotifications{
		unsentUserIDs: []string{"user-1"},
		unsentByUser:  map[string][]notifications.DigestNotification{"user-1": {n1, n2}},
	}
	fz := &fakeZones{byUser: map[string][]watchzones.WatchZone{
		"user-1": {mkZone(t, "zone-1", "Home", true)}, // instant enabled
	}}
	email := &spyEmail{}
	h := newHandler(fp, fn, fz, &fakeState{}, &fakeDevices{}, email, &spyPush{})

	if err := h.RunHourly(context.Background()); err != nil {
		t.Fatalf("RunHourly: %v", err)
	}
	if len(email.sent) != 1 {
		t.Fatalf("emails sent: got %d, want 1", len(email.sent))
	}
	// Both included notifications marked email-sent for dedup.
	if len(fn.marked) != 2 {
		t.Errorf("marked email-sent: got %v, want 2", fn.marked)
	}
}

func TestRunHourly_EmailSendFailureDoesNotMarkSent(t *testing.T) {
	t.Parallel()
	// If the ACS send fails, the notifications must NOT be flipped emailSent — the
	// next cycle must retry. A swallowed send error that marks sent anyway is silent
	// data loss (tc-qvds).
	prefs := profiles.DefaultPreferences()
	prefs.EmailDigestEnabled = true
	p := mkProfile(t, profiles.TierPro, prefs)

	n1 := zoneNotif("uid-A", "zone-1")
	n2 := zoneNotif("uid-B", "")
	n2.WatchZoneID = nil

	fp := &fakeProfiles{byID: map[string]*profiles.UserProfile{"user-1": p}}
	fn := &fakeNotifications{
		unsentUserIDs: []string{"user-1"},
		unsentByUser:  map[string][]notifications.DigestNotification{"user-1": {n1, n2}},
	}
	fz := &fakeZones{byUser: map[string][]watchzones.WatchZone{
		"user-1": {mkZone(t, "zone-1", "Home", true)},
	}}
	email := &spyEmail{sendErr: errors.New("acs send boom")}
	h := newHandler(fp, fn, fz, &fakeState{}, &fakeDevices{}, email, &spyPush{})

	if err := h.RunHourly(context.Background()); err != nil {
		t.Fatalf("RunHourly: %v", err)
	}
	// Send was attempted...
	if len(email.sent) != 1 {
		t.Fatalf("emails attempted: got %d, want 1", len(email.sent))
	}
	// ...but the failure must leave EVERY batched notification unmarked so it is retried.
	if len(fn.marked) != 0 {
		t.Errorf("failed send must not mark anything email-sent: got %v", fn.marked)
	}
}

func TestRunHourly_FreeTierSkipped(t *testing.T) {
	t.Parallel()
	// Free tier has no HourlyDigestEmails entitlement; hourly is paid-only.
	prefs := profiles.DefaultPreferences()
	prefs.EmailDigestEnabled = true
	p := mkProfile(t, profiles.TierFree, prefs)

	fp := &fakeProfiles{byID: map[string]*profiles.UserProfile{"user-1": p}}
	fn := &fakeNotifications{
		unsentUserIDs: []string{"user-1"},
		unsentByUser:  map[string][]notifications.DigestNotification{"user-1": {zoneNotif("uid-A", "zone-1")}},
	}
	fz := &fakeZones{byUser: map[string][]watchzones.WatchZone{
		"user-1": {mkZone(t, "zone-1", "Home", true)},
	}}
	email := &spyEmail{}
	h := newHandler(fp, fn, fz, &fakeState{}, &fakeDevices{}, email, &spyPush{})

	if err := h.RunHourly(context.Background()); err != nil {
		t.Fatalf("RunHourly: %v", err)
	}
	if len(email.sent) != 0 {
		t.Errorf("free tier must NOT get an hourly email: got %d", len(email.sent))
	}
	if len(fn.marked) != 0 {
		t.Errorf("free tier skip must not mark anything sent: got %v", fn.marked)
	}
}

func TestRunHourly_ExpiredPaidTierSkipped(t *testing.T) {
	t.Parallel()
	// A paid tier whose subscription has lapsed (past expiry, no grace) reads as
	// Free, so it has no HourlyDigestEmails entitlement — hourly is paid-only.
	prefs := profiles.DefaultPreferences()
	prefs.EmailDigestEnabled = true
	p := mkProfile(t, profiles.TierPro, prefs)
	past := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) // before the harness clock (2026-02-04)
	p.SubscriptionExpiry = &past

	fp := &fakeProfiles{byID: map[string]*profiles.UserProfile{"user-1": p}}
	fn := &fakeNotifications{
		unsentUserIDs: []string{"user-1"},
		unsentByUser:  map[string][]notifications.DigestNotification{"user-1": {zoneNotif("uid-A", "zone-1")}},
	}
	fz := &fakeZones{byUser: map[string][]watchzones.WatchZone{
		"user-1": {mkZone(t, "zone-1", "Home", true)},
	}}
	email := &spyEmail{}
	h := newHandler(fp, fn, fz, &fakeState{}, &fakeDevices{}, email, &spyPush{})

	if err := h.RunHourly(context.Background()); err != nil {
		t.Fatalf("RunHourly: %v", err)
	}
	if len(email.sent) != 0 {
		t.Errorf("lapsed paid tier must NOT get an hourly email: got %d", len(email.sent))
	}
	if len(fn.marked) != 0 {
		t.Errorf("lapsed paid tier skip must not mark anything sent: got %v", fn.marked)
	}
}

func TestRunHourly_PerZoneInstantGatingExcludesDisabledZone(t *testing.T) {
	t.Parallel()
	prefs := profiles.DefaultPreferences()
	prefs.EmailDigestEnabled = true
	p := mkProfile(t, profiles.TierPro, prefs)

	inZone := zoneNotif("uid-IN", "zone-on")
	offZone := zoneNotif("uid-OFF", "zone-off")

	fp := &fakeProfiles{byID: map[string]*profiles.UserProfile{"user-1": p}}
	fn := &fakeNotifications{
		unsentUserIDs: []string{"user-1"},
		unsentByUser:  map[string][]notifications.DigestNotification{"user-1": {inZone, offZone}},
	}
	fz := &fakeZones{byUser: map[string][]watchzones.WatchZone{
		"user-1": {
			mkZone(t, "zone-on", "On", true),
			mkZone(t, "zone-off", "Off", false), // instant disabled
		},
	}}
	email := &spyEmail{}
	h := newHandler(fp, fn, fz, &fakeState{}, &fakeDevices{}, email, &spyPush{})

	if err := h.RunHourly(context.Background()); err != nil {
		t.Fatalf("RunHourly: %v", err)
	}
	if len(email.sent) != 1 {
		t.Fatalf("emails sent: got %d, want 1", len(email.sent))
	}
	body := email.sent[0].HTMLBody
	if !contains(body, "addr uid-IN") {
		t.Errorf("instant-enabled zone notification should be included:\n%s", body)
	}
	if contains(body, "addr uid-OFF") {
		t.Errorf("instant-disabled zone notification must be excluded:\n%s", body)
	}
	// Only the included notification is marked sent; the excluded one stays unsent
	// so the weekly digest can still pick it up.
	if len(fn.marked) != 1 || fn.marked[0] != "n-uid-IN" {
		t.Errorf("marked: got %v, want [n-uid-IN]", fn.marked)
	}
}

func TestRunHourly_AllZonesDisabledSendsNothing(t *testing.T) {
	t.Parallel()
	prefs := profiles.DefaultPreferences()
	prefs.EmailDigestEnabled = true
	p := mkProfile(t, profiles.TierPro, prefs)

	fp := &fakeProfiles{byID: map[string]*profiles.UserProfile{"user-1": p}}
	fn := &fakeNotifications{
		unsentUserIDs: []string{"user-1"},
		unsentByUser:  map[string][]notifications.DigestNotification{"user-1": {zoneNotif("uid-OFF", "zone-off")}},
	}
	fz := &fakeZones{byUser: map[string][]watchzones.WatchZone{
		"user-1": {mkZone(t, "zone-off", "Off", false)},
	}}
	email := &spyEmail{}
	h := newHandler(fp, fn, fz, &fakeState{}, &fakeDevices{}, email, &spyPush{})

	if err := h.RunHourly(context.Background()); err != nil {
		t.Fatalf("RunHourly: %v", err)
	}
	if len(email.sent) != 0 || len(fn.marked) != 0 {
		t.Errorf("all-zones-disabled: emails=%d marked=%v, want 0 / none", len(email.sent), fn.marked)
	}
}

func TestRunHourly_NoOpSenderDropsEmailButStillMarksSent(t *testing.T) {
	t.Parallel()
	// With a NoOp email sender wired (senders disabled), the cycle still runs and
	// marks notifications sent — the NoOp returns nil, so dedup proceeds.
	prefs := profiles.DefaultPreferences()
	prefs.EmailDigestEnabled = true
	p := mkProfile(t, profiles.TierPro, prefs)

	fp := &fakeProfiles{byID: map[string]*profiles.UserProfile{"user-1": p}}
	fn := &fakeNotifications{
		unsentUserIDs: []string{"user-1"},
		unsentByUser:  map[string][]notifications.DigestNotification{"user-1": {zoneNotif("uid-A", "zone-1")}},
	}
	fz := &fakeZones{byUser: map[string][]watchzones.WatchZone{
		"user-1": {mkZone(t, "zone-1", "Home", true)},
	}}
	h := newHandler(fp, fn, fz, &fakeState{}, &fakeDevices{}, acsemail.NewNoOpSender(), &spyPush{})

	if err := h.RunHourly(context.Background()); err != nil {
		t.Fatalf("RunHourly: %v", err)
	}
	if len(fn.marked) != 1 {
		t.Errorf("NoOp path should still mark sent: got %v", fn.marked)
	}
}

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
