package profiles

import "github.com/AmyDe/town-crier/api-go/internal/erasure"

// CascadeDeleters bundles the per-container erasure steps DELETE /v1/me runs — in
// the fixed order the handler invokes them — before deleting the profile document
// and, last, the Auth0 user. It mirrors the dormant-cleanup worker's cascade so
// account deletion is a complete UK GDPR Art. 17 erasure from either entry point
// (bead tc-qkf2): the Go DELETE /v1/me handler previously deleted only the
// profile and the Auth0 user, leaving watch zones, saved applications,
// notifications, device registrations and the notification-state watermark
// orphaned in Cosmos.
//
// The per-container and offer-code step contracts are the shared erasure
// interfaces (erasure.ChildDeleter, erasure.RedemptionAnonymiser) — the same
// types the dormant-cleanup worker's cascade uses — so the handler can pass these
// deleters straight into erasure.Deleters with no per-package interface drift
// (bead tc-hg65).
type CascadeDeleters struct {
	Notifications       erasure.ChildDeleter
	WatchZones          erasure.ChildDeleter
	SavedApplications   erasure.ChildDeleter
	DeviceRegistrations erasure.ChildDeleter
	NotificationState   erasure.ChildDeleter
	OfferCodes          erasure.RedemptionAnonymiser
}
