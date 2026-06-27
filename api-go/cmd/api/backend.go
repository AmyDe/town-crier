package main

import (
	"strings"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/devicetokens"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/notificationstate"
	"github.com/AmyDe/town-crier/api-go/internal/offercodes"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/savedapplications"
	"github.com/AmyDe/town-crier/api-go/internal/subscriptions"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// storeBackend selects which datastore backs a store. The canonical values are
// backendCosmos (default) and backendPostgres.
type storeBackend int

const (
	// backendCosmos is the default for any flag value other than the exact
	// string "postgres" — including unset and "cosmos" — so prod (flag unset)
	// is never silently flipped off Cosmos.
	backendCosmos storeBackend = iota
	// backendPostgres routes the store to Postgres + PostGIS.
	backendPostgres
)

// postgresBackendValue is the only flag value (whitespace-trimmed) that selects
// Postgres. It is shared by both resolvers so the two flags use identical
// semantics.
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

// choose returns the pg value when backend == backendPostgres (guarding typed-nil),
// or cosmos when backend == backendCosmos (guarding typed-nil). The generic
// parameter T is the concrete pointer type; the return type R is the
// consumer-side interface. Returning R ensures the caller gets a genuine nil
// interface, not a typed-nil pointer boxed in a non-nil interface.
//
// Used by all the per-store chooser helpers below.

// chooseAppStore returns the Applications store for the selected backend as a
// genuine consumer-side interface. When the chosen backend has no backing store
// configured it returns a true nil interface — never a typed-nil pointer boxed
// in a non-nil interface — so newRouter's `appStore != nil` guard correctly
// leaves the application routes unwired on a store-less boot.
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

// chooseProfileStore mirrors chooseAppStore for the user-profile point store.
func chooseProfileStore(backend storeBackend, pg *profiles.PostgresStore, cosmos *profiles.CosmosStore) profiles.Store {
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

// chooseAdminStore mirrors chooseAppStore for the admin profile store.
func chooseAdminStore(backend storeBackend, pg *profiles.PostgresAdminStore, cosmos *profiles.AdminStore) profiles.AdminProfileStore {
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

// chooseDeviceStore mirrors chooseAppStore for the device-registration store.
func chooseDeviceStore(backend storeBackend, pg *devicetokens.PostgresStore, cosmos *devicetokens.CosmosStore) devicetokens.Store {
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

// chooseStateStore mirrors chooseAppStore for the notification-state store.
func chooseStateStore(backend storeBackend, pg *notificationstate.PostgresStore, cosmos *notificationstate.CosmosStore) notificationstate.Store {
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

// chooseNotifStore returns the notification store for the NearbyRoutes
// GetLatestUnreadByApplications path as a notifUnreadReader. It accepts both
// *notifications.CosmosStore (which only has GetLatestUnreadByApplications) and
// *notifications.PostgresStore (which satisfies the full Store) via the narrow
// local interface.
func chooseNotifStore(backend storeBackend, pg *notifications.PostgresStore, cosmos *notifications.CosmosStore) notifUnreadReader {
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

// chooseSavedStore mirrors chooseAppStore for the saved-applications store.
func chooseSavedStore(backend storeBackend, pg *savedapplications.PostgresStore, cosmos *savedapplications.CosmosStore) savedapplications.Store {
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

// chooseOfferStore mirrors chooseAppStore for the offer-code store.
func chooseOfferStore(backend storeBackend, pg *offercodes.PostgresStore, cosmos *offercodes.CosmosStore) offercodes.Store {
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

// chooseAppleNotifStore mirrors chooseAppStore for the Apple-notification
// idempotency store.
func chooseAppleNotifStore(backend storeBackend, pg *subscriptions.PostgresNotificationStore, cosmos *subscriptions.CosmosNotificationStore) subscriptions.Store {
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
