package profiles

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// exportUserData is the GDPR data-export contract returned by GET /v1/me/data,
// including the nested notificationPreferences (with the per-zone array) and
// subscription blocks. JSON keys are camelCase; the tier enum renders as its
// string name. Child collections (watch zones, notifications, saved applications,
// device registrations, offer-code redemptions) are sourced by the per-feature
// stores via the consumer-side ExportReaders (export_readers.go); each is sorted
// deterministically so successive exports are byte-stable, and an absent reader
// (Cosmos-less local boot) yields an empty array — never null.
type exportUserData struct {
	UserID                  string                        `json:"userId"`
	Email                   *string                       `json:"email"`
	NotificationPreferences exportedNotificationPrefs     `json:"notificationPreferences"`
	Subscription            exportedSubscription          `json:"subscription"`
	WatchZones              []ExportedWatchZone           `json:"watchZones"`
	Notifications           []ExportedNotification        `json:"notifications"`
	SavedApplications       []ExportedSavedApplication    `json:"savedApplications"`
	DeviceRegistrations     []ExportedDeviceRegistration  `json:"deviceRegistrations"`
	OfferCodeRedemptions    []ExportedOfferCodeRedemption `json:"offerCodeRedemptions"`
}

type exportedNotificationPrefs struct {
	PushEnabled        bool                     `json:"pushEnabled"`
	DigestDay          weekdayName              `json:"digestDay"`
	EmailDigestEnabled bool                     `json:"emailDigestEnabled"`
	SavedDecisionPush  bool                     `json:"savedDecisionPush"`
	SavedDecisionEmail bool                     `json:"savedDecisionEmail"`
	ZonePreferences    []exportedZonePreference `json:"zonePreferences"`
}

type exportedZonePreference struct {
	ZoneID              string `json:"zoneId"`
	NewApplicationPush  bool   `json:"newApplicationPush"`
	NewApplicationEmail bool   `json:"newApplicationEmail"`
	DecisionPush        bool   `json:"decisionPush"`
	DecisionEmail       bool   `json:"decisionEmail"`
}

type exportedSubscription struct {
	Tier                  string               `json:"tier"`
	ExpiresAt             *platform.DotNetTime `json:"expiresAt"`
	OriginalTransactionID *string              `json:"originalTransactionId"`
	GracePeriodExpiresAt  *platform.DotNetTime `json:"gracePeriodExpiresAt"`
}

// The following child-record shapes are the neutral return types of the
// consumer-side ExportReaders (export_readers.go), so
// they are exported: cmd/api's store adapters build them directly, keeping the
// store -> row mapping out of profiles (which must not import the feature
// packages — see the import-cycle note in export_readers.go).
type ExportedWatchZone struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	Latitude     float64             `json:"latitude"`
	Longitude    float64             `json:"longitude"`
	RadiusMetres float64             `json:"radiusMetres"`
	AuthorityID  int                 `json:"authorityId"`
	CreatedAt    platform.DotNetTime `json:"createdAt"`
}

type ExportedNotification struct {
	ID                     string              `json:"id"`
	ApplicationName        string              `json:"applicationName"`
	WatchZoneID            *string             `json:"watchZoneId"`
	ApplicationAddress     string              `json:"applicationAddress"`
	ApplicationDescription string              `json:"applicationDescription"`
	ApplicationType        *string             `json:"applicationType"`
	AuthorityID            int                 `json:"authorityId"`
	Decision               *string             `json:"decision"`
	PushSent               bool                `json:"pushSent"`
	EmailSent              bool                `json:"emailSent"`
	CreatedAt              platform.DotNetTime `json:"createdAt"`
}

type ExportedSavedApplication struct {
	ApplicationUID string              `json:"applicationUid"`
	SavedAt        platform.DotNetTime `json:"savedAt"`
}

type ExportedDeviceRegistration struct {
	Token        string              `json:"token"`
	Platform     string              `json:"platform"`
	RegisteredAt platform.DotNetTime `json:"registeredAt"`
}

type ExportedOfferCodeRedemption struct {
	Code         string              `json:"code"`
	Tier         string              `json:"tier"`
	DurationDays int                 `json:"durationDays"`
	RedeemedAt   platform.DotNetTime `json:"redeemedAt"`
}

// newExportUserData builds the export contract for a profile, sourcing each child
// collection from its reader and sorting it deterministically so two successive
// exports of the same data are byte-identical (the iOS share fetches the export
// repeatedly). A nil reader (Cosmos-less local boot) leaves that collection as an
// empty, non-nil slice so it serialises as [] rather than null. A reader failure
// is returned so the handler renders a 500 rather than a silently-empty section.
func newExportUserData(ctx context.Context, p *UserProfile, readers ExportReaders) (exportUserData, error) {
	zones := make([]exportedZonePreference, 0, len(p.ZonePreferences))
	for id, z := range p.ZonePreferences {
		zones = append(zones, exportedZonePreference{
			ZoneID:              id,
			NewApplicationPush:  z.NewApplicationPush,
			NewApplicationEmail: z.NewApplicationEmail,
			DecisionPush:        z.DecisionPush,
			DecisionEmail:       z.DecisionEmail,
		})
	}
	// Sort by zoneId so the export is deterministic: ZonePreferences is a Go map,
	// whose iteration order is randomised, which made the array order flake
	// request-to-request (tc-zgnt). A stable order keeps successive exports
	// byte-identical.
	slices.SortFunc(zones, func(a, b exportedZonePreference) int {
		return cmp.Compare(a.ZoneID, b.ZoneID)
	})

	watchZones := []ExportedWatchZone{}
	if readers.WatchZones != nil {
		rows, err := readers.WatchZones.WatchZonesByUser(ctx, p.UserID)
		if err != nil {
			return exportUserData{}, fmt.Errorf("export watch zones: %w", err)
		}
		watchZones = rows
	}
	slices.SortFunc(watchZones, func(a, b ExportedWatchZone) int { return cmp.Compare(a.ID, b.ID) })

	notifs := []ExportedNotification{}
	if readers.Notifications != nil {
		rows, err := readers.Notifications.NotificationsByUser(ctx, p.UserID)
		if err != nil {
			return exportUserData{}, fmt.Errorf("export notifications: %w", err)
		}
		notifs = rows
	}
	slices.SortFunc(notifs, func(a, b ExportedNotification) int { return cmp.Compare(a.ID, b.ID) })

	saved := []ExportedSavedApplication{}
	if readers.SavedApplications != nil {
		rows, err := readers.SavedApplications.SavedApplicationsByUser(ctx, p.UserID)
		if err != nil {
			return exportUserData{}, fmt.Errorf("export saved applications: %w", err)
		}
		saved = rows
	}
	slices.SortFunc(saved, func(a, b ExportedSavedApplication) int { return cmp.Compare(a.ApplicationUID, b.ApplicationUID) })

	devices := []ExportedDeviceRegistration{}
	if readers.DeviceRegistrations != nil {
		rows, err := readers.DeviceRegistrations.DeviceRegistrationsByUser(ctx, p.UserID)
		if err != nil {
			return exportUserData{}, fmt.Errorf("export device registrations: %w", err)
		}
		devices = rows
	}
	slices.SortFunc(devices, func(a, b ExportedDeviceRegistration) int { return cmp.Compare(a.Token, b.Token) })

	codes := []ExportedOfferCodeRedemption{}
	if readers.OfferCodeRedemptions != nil {
		rows, err := readers.OfferCodeRedemptions.OfferCodeRedemptionsByUser(ctx, p.UserID)
		if err != nil {
			return exportUserData{}, fmt.Errorf("export offer-code redemptions: %w", err)
		}
		codes = rows
	}
	slices.SortFunc(codes, func(a, b ExportedOfferCodeRedemption) int { return cmp.Compare(a.Code, b.Code) })

	return exportUserData{
		UserID: p.UserID,
		Email:  p.Email,
		NotificationPreferences: exportedNotificationPrefs{
			PushEnabled:        p.Preferences.PushEnabled,
			DigestDay:          weekdayName(p.Preferences.DigestDay),
			EmailDigestEnabled: p.Preferences.EmailDigestEnabled,
			SavedDecisionPush:  p.Preferences.SavedDecisionPush,
			SavedDecisionEmail: p.Preferences.SavedDecisionEmail,
			ZonePreferences:    zones,
		},
		Subscription: exportedSubscription{
			Tier:                  p.Tier.String(),
			ExpiresAt:             platform.DotNetTimePtr(p.SubscriptionExpiry),
			OriginalTransactionID: p.OriginalTransactionID,
			GracePeriodExpiresAt:  platform.DotNetTimePtr(p.GracePeriodExpiry),
		},
		WatchZones:           watchZones,
		Notifications:        notifs,
		SavedApplications:    saved,
		DeviceRegistrations:  devices,
		OfferCodeRedemptions: codes,
	}, nil
}
