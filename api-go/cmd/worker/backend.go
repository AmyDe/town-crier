package main

import (
	"context"
	"strings"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/devicetokens"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/notificationstate"
	"github.com/AmyDe/town-crier/api-go/internal/offercodes"
	"github.com/AmyDe/town-crier/api-go/internal/polling"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/savedapplications"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// storeBackend selects which datastore backs the stores the worker uses. The
// canonical values are backendCosmos (default) and backendPostgres.
// This duplicates cmd/api/backend.go on purpose: the two package-main binaries
// each own their wiring, so the worker mirrors the API rather than sharing a
// package.
type storeBackend int

const (
	// backendCosmos is the default for any flag value other than the exact
	// string "postgres" — including unset and "cosmos" — so prod (flag unset)
	// is never silently flipped off Cosmos.
	backendCosmos storeBackend = iota
	// backendPostgres routes the store to Postgres + PostGIS.
	backendPostgres
)

// postgresBackendValue is the only flag value (whitespace-trimmed) that
// selects Postgres for any store flag. Shared by all resolvers so the flags
// use identical semantics.
const postgresBackendValue = "postgres"

// resolveBackend maps the APPS_ZONES_BACKEND flag to a storeBackend. Only the
// exact value "postgres" (whitespace-trimmed) selects Postgres; every other
// value, including "" and "cosmos", keeps Cosmos. Kept for backward
// compatibility with the existing APPS_ZONES_BACKEND wiring.
func resolveBackend(flag string) storeBackend {
	if strings.TrimSpace(flag) == postgresBackendValue {
		return backendPostgres
	}
	return backendCosmos
}

// resolveStoreBackend maps the STORE_BACKEND flag to a storeBackend. Only the
// exact value "postgres" (whitespace-trimmed) selects Postgres for ALL stores;
// every other value (including unset) keeps the per-store default. It is a
// dedicated, explicit flag — never inferred from POSTGRES_AUTH — so a future
// prod POSTGRES_AUTH can never silently flip prod stores.
func resolveStoreBackend(flag string) storeBackend {
	if strings.TrimSpace(flag) == postgresBackendValue {
		return backendPostgres
	}
	return backendCosmos
}

// resolveAppsZonesBackend returns backendPostgres when EITHER the
// APPS_ZONES_BACKEND flag OR the STORE_BACKEND flag is "postgres". This
// implements the rule: apps+zones move to Postgres when either flag selects it,
// so a full-cutover STORE_BACKEND=postgres flag also moves apps+zones without
// requiring both flags to be set.
func resolveAppsZonesBackend(appsZonesFlag, storeFlag string) storeBackend {
	if strings.TrimSpace(appsZonesFlag) == postgresBackendValue ||
		strings.TrimSpace(storeFlag) == postgresBackendValue {
		return backendPostgres
	}
	return backendCosmos
}

// pgStores holds every Postgres store instance built from the shared pool.
// The app and zone fields are populated when EITHER APPS_ZONES_BACKEND or
// STORE_BACKEND selects Postgres. All other fields are populated only when
// STORE_BACKEND=postgres (full cutover). Every field is nil when neither flag
// activates it, so builder choosers that check field nil-ness never get a
// typed-nil stored in a non-nil interface.
type pgStores struct {
	// apps+zones: populated by APPS_ZONES_BACKEND=postgres OR STORE_BACKEND=postgres
	app  *applications.PostgresStore
	zone *watchzones.PostgresStore

	// all other stores: populated only by STORE_BACKEND=postgres
	profile      *profiles.PostgresStore
	profileAdmin *profiles.PostgresAdminStore
	notification *notifications.PostgresStore
	notifState   *notificationstate.PostgresStore
	device       *devicetokens.PostgresStore
	savedApp     *savedapplications.PostgresStore
	offerCode    *offercodes.PostgresStore
	pollState    *polling.PostgresPollStateStore
	lease        *polling.PostgresLeaseStore
}

// ── per-store choosers ────────────────────────────────────────────────────────
// Each chooser returns a genuine nil interface when the preferred backend has
// no backing store — never a typed-nil pointer boxed in a non-nil interface —
// so the worker's nil-guard discipline (a missing store leaves the mode
// unwired) holds. The app/zone choosers keep the backend-enum API for backward
// compatibility; all new choosers use the nil-check pattern (pg != nil →
// select pg).

// chooseAppStore returns the Applications store for the selected backend.
func chooseAppStore(backend storeBackend, pg *applications.PostgresStore, cosmos *applications.CosmosStore) applications.Store {
	if backend == backendPostgres {
		if pg == nil {
			return nil
		}
		return pg
	}
	if cosmos == nil {
		return nil
	}
	return cosmos
}

// chooseZoneStore mirrors chooseAppStore for the WatchZones store.
func chooseZoneStore(backend storeBackend, pg *watchzones.PostgresStore, cosmos *watchzones.CosmosStore) watchzones.Store {
	if backend == backendPostgres {
		if pg == nil {
			return nil
		}
		return pg
	}
	if cosmos == nil {
		return nil
	}
	return cosmos
}

// chooseProfileStore returns the point-read profile store, selecting Postgres
// when pg is non-nil (STORE_BACKEND=postgres) and Cosmos otherwise.
func chooseProfileStore(pg *profiles.PostgresStore, cosmos *profiles.CosmosStore) profiles.Store {
	if pg != nil {
		return pg
	}
	if cosmos == nil {
		return nil
	}
	return cosmos
}

// chooseAdminProfileStore returns the admin profile store (cross-partition
// scans: ByDigestDay, Dormant, LapsedPaid, Save), selecting Postgres when pg
// is non-nil and Cosmos otherwise. Both *profiles.PostgresAdminStore and
// *profiles.AdminStore satisfy profiles.AdminProfileStore.
func chooseAdminProfileStore(pg *profiles.PostgresAdminStore, cosmos *profiles.AdminStore) profiles.AdminProfileStore {
	if pg != nil {
		return pg
	}
	if cosmos == nil {
		return nil
	}
	return cosmos
}

// chooseDeviceStore returns the device-registration store.
func chooseDeviceStore(pg *devicetokens.PostgresStore, cosmos *devicetokens.CosmosStore) devicetokens.Store {
	if pg != nil {
		return pg
	}
	if cosmos == nil {
		return nil
	}
	return cosmos
}

// chooseNotifStateStore returns the notification-state store.
func chooseNotifStateStore(pg *notificationstate.PostgresStore, cosmos *notificationstate.CosmosStore) notificationstate.Store {
	if pg != nil {
		return pg
	}
	if cosmos == nil {
		return nil
	}
	return cosmos
}

// chooseSavedStore returns the saved-applications store.
func chooseSavedStore(pg *savedapplications.PostgresStore, cosmos *savedapplications.CosmosStore) savedapplications.Store {
	if pg != nil {
		return pg
	}
	if cosmos == nil {
		return nil
	}
	return cosmos
}

// chooseOfferStore returns the offer-code store.
func chooseOfferStore(pg *offercodes.PostgresStore, cosmos *offercodes.CosmosStore) offercodes.Store {
	if pg != nil {
		return pg
	}
	if cosmos == nil {
		return nil
	}
	return cosmos
}

// notifDigestWriter is the consumer-side interface satisfied by both
// *notifications.PostgresStore and *notifications.DigestStore. It is the
// minimal slice needed by notifydispatch.Enqueuer / DecisionDispatcher (create
// + dedup read) and digest.Handler (read + email-sent mark).
type notifDigestWriter interface {
	GetByUserAndApplication(ctx context.Context, userID, applicationUID string, authorityID int, eventType notifications.EventType) (*notifications.DigestNotification, error)
	Create(ctx context.Context, n notifications.DigestNotification) error
	ByUserSince(ctx context.Context, userID string, since time.Time) ([]notifications.DigestNotification, error)
	UnsentEmailsByUser(ctx context.Context, userID string) ([]notifications.DigestNotification, error)
	UserIDsWithUnsentEmails(ctx context.Context) ([]string, error)
	MarkEmailSent(ctx context.Context, n notifications.DigestNotification) error
}

// chooseNotifDigestStore returns the notification store for the fan-out and
// digest paths, selecting Postgres when pg is non-nil and the Cosmos DigestStore
// otherwise. Both satisfy notifDigestWriter.
func chooseNotifDigestStore(pg *notifications.PostgresStore, cosmos *notifications.DigestStore) notifDigestWriter {
	if pg != nil {
		return pg
	}
	if cosmos == nil {
		return nil
	}
	return cosmos
}
