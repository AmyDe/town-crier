package profiles

import "github.com/AmyDe/town-crier/api-go/internal/erasure"

// CascadeDeleters are the per-user erasure steps DELETE /v1/me runs before
// deleting the profile and, last, the Auth0 user. The field types are the shared
// erasure interfaces, so they pass straight into erasure.Deleters.
type CascadeDeleters struct {
	Notifications       erasure.ChildDeleter
	WatchZones          erasure.ChildDeleter
	SavedApplications   erasure.ChildDeleter
	DeviceRegistrations erasure.ChildDeleter
	NotificationState   erasure.ChildDeleter
	OfferCodes          erasure.RedemptionAnonymiser
}
