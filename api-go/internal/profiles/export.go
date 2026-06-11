package profiles

import "time"

// exportUserData is the GDPR data-export contract returned by GET /v1/me/data.
// It mirrors .NET's ExportUserDataResult, including the nested
// notificationPreferences (with the per-zone array) and subscription blocks.
// JSON keys are camelCase to match the web serializer; the tier enum renders as
// its string name. Child collections (watch zones, notifications, saved
// applications, device registrations, offer-code redemptions) are sourced by
// stores that arrive in later iterations; until then they serialise as empty
// arrays — never null — to match .NET's IReadOnlyList<>.ToList() empty result.
type exportUserData struct {
	UserID                  string                        `json:"userId"`
	Email                   *string                       `json:"email"`
	NotificationPreferences exportedNotificationPrefs     `json:"notificationPreferences"`
	Subscription            exportedSubscription          `json:"subscription"`
	WatchZones              []exportedWatchZone           `json:"watchZones"`
	Notifications           []exportedNotification        `json:"notifications"`
	SavedApplications       []exportedSavedApplication    `json:"savedApplications"`
	DeviceRegistrations     []exportedDeviceRegistration  `json:"deviceRegistrations"`
	OfferCodeRedemptions    []exportedOfferCodeRedemption `json:"offerCodeRedemptions"`
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
	Tier                  string     `json:"tier"`
	ExpiresAt             *time.Time `json:"expiresAt"`
	OriginalTransactionID *string    `json:"originalTransactionId"`
	GracePeriodExpiresAt  *time.Time `json:"gracePeriodExpiresAt"`
}

// The following child-record shapes match .NET's Exported* records. They have no
// data source in iteration 3 (their stores land later), so the export always
// renders them as empty arrays; the structs pin the contract for when the
// sources arrive.
type exportedWatchZone struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Latitude     float64   `json:"latitude"`
	Longitude    float64   `json:"longitude"`
	RadiusMetres float64   `json:"radiusMetres"`
	AuthorityID  int       `json:"authorityId"`
	CreatedAt    time.Time `json:"createdAt"`
}

type exportedNotification struct {
	ID                     string    `json:"id"`
	ApplicationName        string    `json:"applicationName"`
	WatchZoneID            *string   `json:"watchZoneId"`
	ApplicationAddress     string    `json:"applicationAddress"`
	ApplicationDescription string    `json:"applicationDescription"`
	ApplicationType        *string   `json:"applicationType"`
	AuthorityID            int       `json:"authorityId"`
	Decision               *string   `json:"decision"`
	PushSent               bool      `json:"pushSent"`
	EmailSent              bool      `json:"emailSent"`
	CreatedAt              time.Time `json:"createdAt"`
}

type exportedSavedApplication struct {
	ApplicationUID string    `json:"applicationUid"`
	SavedAt        time.Time `json:"savedAt"`
}

type exportedDeviceRegistration struct {
	Token        string    `json:"token"`
	Platform     string    `json:"platform"`
	RegisteredAt time.Time `json:"registeredAt"`
}

type exportedOfferCodeRedemption struct {
	Code         string    `json:"code"`
	Tier         string    `json:"tier"`
	DurationDays int       `json:"durationDays"`
	RedeemedAt   time.Time `json:"redeemedAt"`
}

// newExportUserData builds the export contract for a profile, with child
// collections initialised as empty (non-nil) slices so they serialise as [].
func newExportUserData(p *UserProfile) exportUserData {
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
			ExpiresAt:             p.SubscriptionExpiry,
			OriginalTransactionID: p.OriginalTransactionID,
			GracePeriodExpiresAt:  p.GracePeriodExpiry,
		},
		WatchZones:           []exportedWatchZone{},
		Notifications:        []exportedNotification{},
		SavedApplications:    []exportedSavedApplication{},
		DeviceRegistrations:  []exportedDeviceRegistration{},
		OfferCodeRedemptions: []exportedOfferCodeRedemption{},
	}
}
