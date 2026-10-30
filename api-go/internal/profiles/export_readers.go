package profiles

import "context"

// The feature stores the GDPR export reads cannot be imported here without an
// import cycle (offercodes already imports profiles), so cmd/api supplies
// adapters that return the Exported* rows.

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
// the export's child collections, mirroring CascadeDeleters. A nil field renders
// its collection as [] (never null).
type ExportReaders struct {
	WatchZones           WatchZoneReader
	Notifications        NotificationReader
	SavedApplications    SavedApplicationReader
	DeviceRegistrations  DeviceRegistrationReader
	OfferCodeRedemptions OfferCodeRedemptionReader
}
