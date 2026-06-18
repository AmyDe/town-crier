package profiles

import "context"

// The GDPR export (GET /v1/me/data) populates its child collections from the
// per-feature stores. Those stores live in packages that cannot be imported here
// without a cycle: package offercodes already imports profiles
// (profiles.ParseSubscriptionTier), so profiles importing offercodes (or, by the
// same DI symmetry, watchzones / notifications / savedapplications / devicetokens)
// would close a loop. The readers below are therefore consumer-side interfaces,
// declared where they are used, returning profiles-local row structs (the
// Exported* types in export.go). cmd/api supplies thin adapters that map each
// store's records to those row types, mirroring the CascadeDeleters pattern
// (cascade.go + cmd/api/main.go) exactly.
//
// Each reader is single-method and returns the already-shaped export rows so the
// export does no per-store mapping itself.

// WatchZoneReader reads a user's watch zones for the export.
type WatchZoneReader interface {
	WatchZonesByUser(ctx context.Context, userID string) ([]ExportedWatchZone, error)
}

// NotificationReader reads a user's dispatched notifications for the export.
type NotificationReader interface {
	NotificationsByUser(ctx context.Context, userID string) ([]ExportedNotification, error)
}

// SavedApplicationReader reads a user's saved applications for the export.
type SavedApplicationReader interface {
	SavedApplicationsByUser(ctx context.Context, userID string) ([]ExportedSavedApplication, error)
}

// DeviceRegistrationReader reads a user's device registrations for the export.
type DeviceRegistrationReader interface {
	DeviceRegistrationsByUser(ctx context.Context, userID string) ([]ExportedDeviceRegistration, error)
}

// OfferCodeRedemptionReader reads the offer codes a user redeemed for the export.
type OfferCodeRedemptionReader interface {
	OfferCodeRedemptionsByUser(ctx context.Context, userID string) ([]ExportedOfferCodeRedemption, error)
}

// ExportReaders bundles the per-collection readers GET /v1/me/data uses to source
// the export's child collections, mirroring CascadeDeleters. It is populated in
// cmd/api under the same Cosmos-configured guard as profiles.Routes; on a
// Cosmos-less local boot the fields are nil and the export renders every
// collection as [] (never null) — newExportUserData skips a nil reader.
type ExportReaders struct {
	WatchZones           WatchZoneReader
	Notifications        NotificationReader
	SavedApplications    SavedApplicationReader
	DeviceRegistrations  DeviceRegistrationReader
	OfferCodeRedemptions OfferCodeRedemptionReader
}
