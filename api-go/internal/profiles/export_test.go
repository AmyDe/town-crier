package profiles

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// The export readers are hand-written per-collection doubles; each returns the
// profiles-local row slice the matching reader yields (the readers already return
// profiles-local row structs, so the export maps nothing further). They are
// defined below the constructor that assembles them into an ExportReaders bundle.

func dnt(t *testing.T, s string) platform.DotNetTime {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time %q: %v", s, err)
	}
	return platform.DotNetTime(parsed)
}

func populatedReaders(t *testing.T) ExportReaders {
	t.Helper()
	at := func(s string) platform.DotNetTime { return dnt(t, s) }
	wzID := "zone-a"
	appType := "Full"
	decision := "Permitted"
	return ExportReaders{
		WatchZones: fakeWatchZonesReader{rows: []ExportedWatchZone{
			{ID: "zone-b", Name: "Bravo", Latitude: 51.5, Longitude: -0.1, RadiusMetres: 500, AuthorityID: 22, CreatedAt: at("2026-02-02T00:00:00Z")},
			{ID: "zone-a", Name: "Alpha", Latitude: 52.5, Longitude: -1.1, RadiusMetres: 1000, AuthorityID: 11, CreatedAt: at("2026-02-01T00:00:00Z")},
		}},
		Notifications: fakeNotificationsReader{rows: []ExportedNotification{
			{ID: "n-2", ApplicationName: "App Two", WatchZoneID: &wzID, ApplicationAddress: "2 St", ApplicationDescription: "two", ApplicationType: &appType, AuthorityID: 22, Decision: &decision, PushSent: true, EmailSent: false, CreatedAt: at("2026-03-02T00:00:00Z")},
			{ID: "n-1", ApplicationName: "App One", ApplicationAddress: "1 St", ApplicationDescription: "one", AuthorityID: 11, PushSent: true, EmailSent: true, CreatedAt: at("2026-03-01T00:00:00Z")},
		}},
		SavedApplications: fakeSavedReader{rows: []ExportedSavedApplication{
			{ApplicationUID: "22/0002", SavedAt: at("2026-04-02T00:00:00Z")},
			{ApplicationUID: "11/0001", SavedAt: at("2026-04-01T00:00:00Z")},
		}},
		DeviceRegistrations: fakeDevicesReader{rows: []ExportedDeviceRegistration{
			{Token: "tok-z", Platform: "Android", RegisteredAt: at("2026-05-02T00:00:00Z")},
			{Token: "tok-a", Platform: "Ios", RegisteredAt: at("2026-05-01T00:00:00Z")},
		}},
		OfferCodeRedemptions: fakeCodesReader{rows: []ExportedOfferCodeRedemption{
			{Code: "ZZZZ", Tier: "Pro", DurationDays: 30, RedeemedAt: at("2026-06-02T00:00:00Z")},
			{Code: "AAAA", Tier: "Personal", DurationDays: 14, RedeemedAt: at("2026-06-01T00:00:00Z")},
		}},
	}
}

type fakeWatchZonesReader struct {
	rows []ExportedWatchZone
	err  error
}

func (f fakeWatchZonesReader) WatchZonesByUser(_ context.Context, _ string) ([]ExportedWatchZone, error) {
	return f.rows, f.err
}

type fakeNotificationsReader struct{ rows []ExportedNotification }

func (f fakeNotificationsReader) NotificationsByUser(_ context.Context, _ string) ([]ExportedNotification, error) {
	return f.rows, nil
}

type fakeSavedReader struct{ rows []ExportedSavedApplication }

func (f fakeSavedReader) SavedApplicationsByUser(_ context.Context, _ string) ([]ExportedSavedApplication, error) {
	return f.rows, nil
}

type fakeDevicesReader struct{ rows []ExportedDeviceRegistration }

func (f fakeDevicesReader) DeviceRegistrationsByUser(_ context.Context, _ string) ([]ExportedDeviceRegistration, error) {
	return f.rows, nil
}

type fakeCodesReader struct{ rows []ExportedOfferCodeRedemption }

func (f fakeCodesReader) OfferCodeRedemptionsByUser(_ context.Context, _ string) ([]ExportedOfferCodeRedemption, error) {
	return f.rows, nil
}

func testExportProfile() *UserProfile {
	return &UserProfile{
		UserID:       "auth0|abc",
		Preferences:  DefaultPreferences(),
		LastActiveAt: time.Now(),
	}
}

// TestNewExportUserData_PopulatedFromReaders pins the GDPR export populating every
// child collection from the readers, each sorted deterministically so successive
// exports are byte-stable, with the shapes the Privacy Policy promises.
func TestNewExportUserData_PopulatedFromReaders(t *testing.T) {
	t.Parallel()

	export, err := newExportUserData(context.Background(), testExportProfile(), populatedReaders(t))
	if err != nil {
		t.Fatalf("newExportUserData: %v", err)
	}

	if got := zoneIDs(export.WatchZones); !equalStrings(got, []string{"zone-a", "zone-b"}) {
		t.Errorf("watchZones not sorted by id: %v", got)
	}
	if got := notifIDs(export.Notifications); !equalStrings(got, []string{"n-1", "n-2"}) {
		t.Errorf("notifications not sorted by id: %v", got)
	}
	if got := savedUIDs(export.SavedApplications); !equalStrings(got, []string{"11/0001", "22/0002"}) {
		t.Errorf("savedApplications not sorted by applicationUid: %v", got)
	}
	if got := deviceTokens(export.DeviceRegistrations); !equalStrings(got, []string{"tok-a", "tok-z"}) {
		t.Errorf("deviceRegistrations not sorted by token: %v", got)
	}
	if got := codeStrings(export.OfferCodeRedemptions); !equalStrings(got, []string{"AAAA", "ZZZZ"}) {
		t.Errorf("offerCodeRedemptions not sorted by code: %v", got)
	}

	// Byte-stability: marshalling the same export twice (and re-running the build)
	// must yield identical bytes — the export is read repeatedly per the iOS share.
	first := mustMarshal(t, export)
	again, err := newExportUserData(context.Background(), testExportProfile(), populatedReaders(t))
	if err != nil {
		t.Fatalf("newExportUserData (repeat): %v", err)
	}
	if second := mustMarshal(t, again); first != second {
		t.Errorf("export not byte-stable across calls:\n first=%s\nsecond=%s", first, second)
	}
}

// TestNewExportUserData_PopulatedShapes asserts the wire shape (camelCase keys,
// .NET timestamp format, nested optional fields) of one fully-shaped row per
// collection so the contract the Privacy Policy promises is exact.
func TestNewExportUserData_PopulatedShapes(t *testing.T) {
	t.Parallel()

	export, err := newExportUserData(context.Background(), testExportProfile(), populatedReaders(t))
	if err != nil {
		t.Fatalf("newExportUserData: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(mustMarshal(t, export)), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	wz := got["watchZones"].([]any)[0].(map[string]any) // sorted: zone-a first
	if wz["id"] != "zone-a" || wz["name"] != "Alpha" || wz["radiusMetres"] != 1000.0 || wz["authorityId"] != 11.0 {
		t.Errorf("watchZone shape wrong: %v", wz)
	}
	if wz["createdAt"] != "2026-02-01T00:00:00+00:00" {
		t.Errorf("watchZone createdAt wire format: got %v", wz["createdAt"])
	}

	n := got["notifications"].([]any)[1].(map[string]any) // sorted: n-1, n-2 -> index 1 is n-2 (has the optionals)
	if n["id"] != "n-2" || n["watchZoneId"] != "zone-a" || n["applicationType"] != "Full" || n["decision"] != "Permitted" {
		t.Errorf("notification shape wrong: %v", n)
	}
	if n["pushSent"] != true || n["emailSent"] != false {
		t.Errorf("notification flags wrong: %v", n)
	}

	sa := got["savedApplications"].([]any)[0].(map[string]any)
	if sa["applicationUid"] != "11/0001" || sa["savedAt"] != "2026-04-01T00:00:00+00:00" {
		t.Errorf("savedApplication shape wrong: %v", sa)
	}

	dr := got["deviceRegistrations"].([]any)[0].(map[string]any)
	if dr["token"] != "tok-a" || dr["platform"] != "Ios" || dr["registeredAt"] != "2026-05-01T00:00:00+00:00" {
		t.Errorf("deviceRegistration shape wrong: %v", dr)
	}

	oc := got["offerCodeRedemptions"].([]any)[0].(map[string]any)
	if oc["code"] != "AAAA" || oc["tier"] != "Personal" || oc["durationDays"] != 14.0 || oc["redeemedAt"] != "2026-06-01T00:00:00+00:00" {
		t.Errorf("offerCodeRedemption shape wrong: %v", oc)
	}
}

// TestNewExportUserData_NilReadersYieldEmptyArrays guards the Cosmos-less local
// boot: when the readers are absent (zero-valued bundle) every child collection
// must still render as [] — never null.
func TestNewExportUserData_NilReadersYieldEmptyArrays(t *testing.T) {
	t.Parallel()

	export, err := newExportUserData(context.Background(), testExportProfile(), ExportReaders{})
	if err != nil {
		t.Fatalf("newExportUserData: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(mustMarshal(t, export)), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, k := range []string{"watchZones", "notifications", "savedApplications", "deviceRegistrations", "offerCodeRedemptions"} {
		arr, ok := got[k].([]any)
		if !ok {
			t.Errorf("%s must be [] (array), got %v (%T)", k, got[k], got[k])
			continue
		}
		if len(arr) != 0 {
			t.Errorf("%s must be empty with nil readers, got %v", k, arr)
		}
	}
}

// TestNewExportUserData_ReaderErrorPropagates ensures a store failure surfaces as
// an error (the handler maps it to a 500) rather than a silent empty collection.
func TestNewExportUserData_ReaderErrorPropagates(t *testing.T) {
	t.Parallel()

	readers := ExportReaders{WatchZones: fakeWatchZonesReader{err: errBoom}}
	if _, err := newExportUserData(context.Background(), testExportProfile(), readers); err == nil {
		t.Fatal("expected an error when a reader fails, got nil")
	}
}

var errBoom = &boomError{}

type boomError struct{}

func (*boomError) Error() string { return "boom" }

func mustMarshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func zoneIDs(zs []ExportedWatchZone) []string {
	out := make([]string, len(zs))
	for i, z := range zs {
		out[i] = z.ID
	}
	return out
}

func notifIDs(ns []ExportedNotification) []string {
	out := make([]string, len(ns))
	for i, n := range ns {
		out[i] = n.ID
	}
	return out
}

func savedUIDs(ss []ExportedSavedApplication) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = s.ApplicationUID
	}
	return out
}

func deviceTokens(ds []ExportedDeviceRegistration) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.Token
	}
	return out
}

func codeStrings(cs []ExportedOfferCodeRedemption) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Code
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestNewExportUserData_ZonePreferencesSortedByZoneID pins the fix for tc-zgnt:
// the GDPR export builds zonePreferences by ranging a Go map, whose iteration
// order is randomised, so without an explicit sort the array order flaked
// request-to-request. The export must emit the per-zone preferences in a stable
// order — sorted by zoneId — so two successive exports of the same profile are
// byte-identical. Building the map in reverse-sorted insertion order, and
// asserting the exported slice is forward-sorted, fails reliably until the sort
// lands (Go randomises map ranging, so an unsorted build matches the sorted
// expectation only by chance).
func TestNewExportUserData_ZonePreferencesSortedByZoneID(t *testing.T) {
	t.Parallel()

	p := &UserProfile{
		UserID:       "auth0|abc",
		Preferences:  DefaultPreferences(),
		LastActiveAt: time.Now(),
		ZonePreferences: map[string]ZonePreferences{
			"cb5224db": DefaultZonePreferences(),
			"eb39413d": DefaultZonePreferences(),
			"4abf25b2": DefaultZonePreferences(),
		},
	}

	want := []string{"4abf25b2", "cb5224db", "eb39413d"}
	export, err := newExportUserData(context.Background(), p, ExportReaders{})
	if err != nil {
		t.Fatalf("newExportUserData: %v", err)
	}
	got := make([]string, 0, len(export.NotificationPreferences.ZonePreferences))
	for _, z := range export.NotificationPreferences.ZonePreferences {
		got = append(got, z.ZoneID)
	}

	if len(got) != len(want) {
		t.Fatalf("zonePreferences length: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("zonePreferences not sorted by zoneId: got %v, want %v", got, want)
		}
	}
}
