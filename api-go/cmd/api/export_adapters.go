package main

import (
	"context"

	"github.com/AmyDe/town-crier/api-go/internal/devicetokens"
	"github.com/AmyDe/town-crier/api-go/internal/erasure"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/offercodes"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/savedapplications"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// gdprWatchZoneWiring binds the GDPR erasure cascade (DELETE /v1/me) and the data
// export (GET /v1/me/data) to the SAME flag-selected watch-zone store the routes
// use, so a Postgres-resident user's watch zones are erased and exported in both
// Cosmos and Postgres modes — not silently missed by the always-Cosmos store
// (bead tc-s8g1). watchzones.Store satisfies erasure.ChildDeleter directly (it
// exposes DeleteAllByUserID), so the cascade needs no adapter; only the export
// needs the watchZoneExportReader row mapping.
//
// A nil store (no backing configured) yields genuine nil wiring so the caller's
// atomic cascade/export guards leave the GDPR paths unbuilt rather than holding a
// nil deleter the cascade would dereference.
func gdprWatchZoneWiring(store watchzones.Store) (erasure.ChildDeleter, profiles.WatchZoneReader) {
	if store == nil {
		return nil, nil
	}
	return store, watchZoneExportReader{store: store}
}

// The GDPR export (GET /v1/me/data) sources its child collections from the
// per-feature stores. profiles must not import those packages (offercodes already
// imports profiles, so a back-import would close a cycle — see
// profiles/export_readers.go), so the store -> export-row mapping lives here, in
// cmd/api, as thin adapters that satisfy the consumer-side profiles reader
// interfaces. This mirrors the cascade adapters built in main.go (the
// erasure.ChildDeleter wiring): the wiring layer is the one place that depends on
// both the stores and the profiles row types.

// watchZoneExportReader adapts the watch-zone store to profiles.WatchZoneReader.
// It holds the consumer-side watchzones.Store interface — not the concrete Cosmos
// store — so GET /v1/me/data exports a user's watch zones from whichever backend
// the APPS_ZONES_BACKEND flag selects (Postgres on dev), never silently missing a
// Postgres-resident user's zones (bead tc-s8g1).
type watchZoneExportReader struct{ store watchzones.Store }

func (r watchZoneExportReader) WatchZonesByUser(ctx context.Context, userID string) ([]profiles.ExportedWatchZone, error) {
	zones, err := r.store.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	rows := make([]profiles.ExportedWatchZone, 0, len(zones))
	for _, z := range zones {
		rows = append(rows, profiles.ExportedWatchZone{
			ID:           z.ID,
			Name:         z.Name,
			Latitude:     z.Latitude,
			Longitude:    z.Longitude,
			RadiusMetres: z.RadiusMetres,
			AuthorityID:  z.AuthorityID,
			CreatedAt:    platform.DotNetTime(z.CreatedAt),
		})
	}
	return rows, nil
}

// notificationExportReader adapts the notifications digest store to
// profiles.NotificationReader. It reads the full Notifications-container document
// (AllByUser) so every exported field is carried.
type notificationExportReader struct{ store *notifications.DigestStore }

func (r notificationExportReader) NotificationsByUser(ctx context.Context, userID string) ([]profiles.ExportedNotification, error) {
	notifs, err := r.store.AllByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	rows := make([]profiles.ExportedNotification, 0, len(notifs))
	for _, n := range notifs {
		rows = append(rows, profiles.ExportedNotification{
			ID:                     n.ID,
			ApplicationName:        n.ApplicationName,
			WatchZoneID:            n.WatchZoneID,
			ApplicationAddress:     n.ApplicationAddress,
			ApplicationDescription: n.ApplicationDescription,
			ApplicationType:        n.ApplicationType,
			AuthorityID:            n.AuthorityID,
			Decision:               n.Decision,
			PushSent:               n.PushSent,
			EmailSent:              n.EmailSent,
			CreatedAt:              platform.DotNetTime(n.CreatedAt),
		})
	}
	return rows, nil
}

// savedApplicationExportReader adapts the saved-application store to
// profiles.SavedApplicationReader.
type savedApplicationExportReader struct {
	store *savedapplications.CosmosStore
}

func (r savedApplicationExportReader) SavedApplicationsByUser(ctx context.Context, userID string) ([]profiles.ExportedSavedApplication, error) {
	saved, err := r.store.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	rows := make([]profiles.ExportedSavedApplication, 0, len(saved))
	for _, s := range saved {
		rows = append(rows, profiles.ExportedSavedApplication{
			ApplicationUID: s.ApplicationUID,
			SavedAt:        platform.DotNetTime(s.SavedAt),
		})
	}
	return rows, nil
}

// deviceRegistrationExportReader adapts the device-token store to
// profiles.DeviceRegistrationReader.
type deviceRegistrationExportReader struct{ store *devicetokens.CosmosStore }

func (r deviceRegistrationExportReader) DeviceRegistrationsByUser(ctx context.Context, userID string) ([]profiles.ExportedDeviceRegistration, error) {
	regs, err := r.store.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	rows := make([]profiles.ExportedDeviceRegistration, 0, len(regs))
	for _, d := range regs {
		rows = append(rows, profiles.ExportedDeviceRegistration{
			Token:        d.Token,
			Platform:     d.Platform.String(),
			RegisteredAt: platform.DotNetTime(d.RegisteredAt),
		})
	}
	return rows, nil
}

// offerCodeExportReader adapts the offer-code store to
// profiles.OfferCodeRedemptionReader. A redeemed-by-user code always has a
// RedeemedAt set, but a nil is guarded (it serialises as the zero instant) so a
// malformed document can never panic the export.
type offerCodeExportReader struct{ store *offercodes.CosmosStore }

func (r offerCodeExportReader) OfferCodeRedemptionsByUser(ctx context.Context, userID string) ([]profiles.ExportedOfferCodeRedemption, error) {
	codes, err := r.store.RedeemedByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	rows := make([]profiles.ExportedOfferCodeRedemption, 0, len(codes))
	for _, c := range codes {
		var redeemedAt platform.DotNetTime
		if c.RedeemedAt != nil {
			redeemedAt = platform.DotNetTime(*c.RedeemedAt)
		}
		rows = append(rows, profiles.ExportedOfferCodeRedemption{
			Code:         c.Code,
			Tier:         c.Tier.String(),
			DurationDays: c.DurationDays,
			RedeemedAt:   redeemedAt,
		})
	}
	return rows, nil
}
