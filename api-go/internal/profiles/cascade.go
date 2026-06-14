package profiles

import "context"

// ChildDeleter erases every document a single per-user container holds for the
// account being deleted. The per-container cascade stores expose this exact
// method, so notifications.DeleteStore and the watchzones / savedapplications /
// devicetokens CosmosStores satisfy it directly; the notification-state store is
// bridged by a one-line adapter in the api wiring, since its method is
// DeleteByUserID. Each store tolerates a 404 on an individual delete internally,
// so any error returned here is real and must abort the cascade.
type ChildDeleter interface {
	DeleteAllByUserID(ctx context.Context, userID string) error
}

// CascadeDeleters bundles the per-container erasure steps DELETE /v1/me runs — in
// the fixed order the handler invokes them — before deleting the profile document
// and, last, the Auth0 user. It mirrors the dormant-cleanup worker's cascade so
// account deletion is a complete UK GDPR Art. 17 erasure from either entry point
// (bead tc-qkf2): the Go DELETE /v1/me handler previously deleted only the
// profile and the Auth0 user, leaving watch zones, saved applications,
// notifications, device registrations and the notification-state watermark
// orphaned in Cosmos.
type CascadeDeleters struct {
	Notifications       ChildDeleter
	WatchZones          ChildDeleter
	SavedApplications   ChildDeleter
	DeviceRegistrations ChildDeleter
	NotificationState   ChildDeleter
}
