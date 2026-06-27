package main

import (
	"testing"
	"time"

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

// TestResolveBackend pins the APPS_ZONES_BACKEND contract: only the exact value
// "postgres" (whitespace-trimmed) selects Postgres; every other value, including
// unset and "cosmos", keeps Cosmos so prod (flag unset) is never silently
// flipped, and a non-canonical casing is treated as "any other value".
func TestResolveBackend(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		flag string
		want storeBackend
	}{
		{"exact postgres selects postgres", "postgres", backendPostgres},
		{"surrounding whitespace is trimmed", "  postgres\t", backendPostgres},
		{"unset defaults to cosmos", "", backendCosmos},
		{"explicit cosmos stays cosmos", "cosmos", backendCosmos},
		{"uppercase is not the canonical value", "Postgres", backendCosmos},
		{"junk defaults to cosmos", "nonsense", backendCosmos},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := resolveBackend(tc.flag); got != tc.want {
				t.Fatalf("resolveBackend(%q) = %v, want %v", tc.flag, got, tc.want)
			}
		})
	}
}

// TestChooseAppStore covers both the backend selection and the typed-nil trap:
// when the chosen backend has no backing store the chooser must return a GENUINE
// nil interface (not a typed-nil pointer boxed in a non-nil interface), so
// newRouter's `appStore != nil` guard leaves the application routes unwired.
func TestChooseAppStore(t *testing.T) {
	t.Parallel()

	cosmos := applications.NewCosmosStore(newFakeItems())
	pg := applications.NewPostgresStore(nil) // querier is never touched in these tests

	t.Run("postgres backend returns the postgres store", func(t *testing.T) {
		t.Parallel()
		got := chooseAppStore(backendPostgres, pg, cosmos)
		if _, ok := got.(*applications.PostgresStore); !ok {
			t.Fatalf("got %T, want *applications.PostgresStore", got)
		}
	})

	t.Run("cosmos backend returns the cosmos store", func(t *testing.T) {
		t.Parallel()
		got := chooseAppStore(backendCosmos, pg, cosmos)
		if _, ok := got.(*applications.CosmosStore); !ok {
			t.Fatalf("got %T, want *applications.CosmosStore", got)
		}
	})

	t.Run("postgres backend with no pool yields a genuine nil interface", func(t *testing.T) {
		t.Parallel()
		if got := chooseAppStore(backendPostgres, nil, cosmos); got != nil {
			t.Fatalf("got %T (non-nil interface), want a genuine nil so routes stay unwired", got)
		}
	})

	t.Run("cosmos backend with no container yields a genuine nil interface", func(t *testing.T) {
		t.Parallel()
		if got := chooseAppStore(backendCosmos, pg, nil); got != nil {
			t.Fatalf("got %T (non-nil interface), want a genuine nil so routes stay unwired", got)
		}
	})
}

// TestChooseZoneStore mirrors TestChooseAppStore for the watch-zone store.
func TestChooseZoneStore(t *testing.T) {
	t.Parallel()

	cosmos := watchzones.NewCosmosStore(newFakeItems())
	pg := watchzones.NewPostgresStore(nil)

	t.Run("postgres backend returns the postgres store", func(t *testing.T) {
		t.Parallel()
		got := chooseZoneStore(backendPostgres, pg, cosmos)
		if _, ok := got.(*watchzones.PostgresStore); !ok {
			t.Fatalf("got %T, want *watchzones.PostgresStore", got)
		}
	})

	t.Run("cosmos backend returns the cosmos store", func(t *testing.T) {
		t.Parallel()
		got := chooseZoneStore(backendCosmos, pg, cosmos)
		if _, ok := got.(*watchzones.CosmosStore); !ok {
			t.Fatalf("got %T, want *watchzones.CosmosStore", got)
		}
	})

	t.Run("postgres backend with no pool yields a genuine nil interface", func(t *testing.T) {
		t.Parallel()
		if got := chooseZoneStore(backendPostgres, nil, cosmos); got != nil {
			t.Fatalf("got %T (non-nil interface), want a genuine nil so routes stay unwired", got)
		}
	})

	t.Run("cosmos backend with no container yields a genuine nil interface", func(t *testing.T) {
		t.Parallel()
		if got := chooseZoneStore(backendCosmos, pg, nil); got != nil {
			t.Fatalf("got %T (non-nil interface), want a genuine nil so routes stay unwired", got)
		}
	})
}

// TestResolveStoreBackend pins the STORE_BACKEND contract: only the exact
// string "postgres" (whitespace-trimmed) selects Postgres for ALL stores;
// every other value (including unset) keeps the per-store default so prod
// (flag unset) is never silently flipped.
func TestResolveStoreBackend(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		flag string
		want storeBackend
	}{
		{"exact postgres selects postgres", "postgres", backendPostgres},
		{"surrounding whitespace is trimmed", "  postgres\t", backendPostgres},
		{"unset defaults to cosmos", "", backendCosmos},
		{"explicit cosmos stays cosmos", "cosmos", backendCosmos},
		{"uppercase is not the canonical value", "Postgres", backendCosmos},
		{"junk defaults to cosmos", "nonsense", backendCosmos},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := resolveStoreBackend(tc.flag); got != tc.want {
				t.Fatalf("resolveStoreBackend(%q) = %v, want %v", tc.flag, got, tc.want)
			}
		})
	}
}

// TestChooseProfileStore verifies backend selection and the typed-nil trap for
// the profiles point store.
func TestChooseProfileStore(t *testing.T) {
	t.Parallel()

	cosmos := profiles.NewCosmosStore(newFakeItems())
	pg := profiles.NewPostgresStore(nil)

	t.Run("postgres backend returns the postgres store", func(t *testing.T) {
		t.Parallel()
		got := chooseProfileStore(backendPostgres, pg, cosmos)
		if _, ok := got.(*profiles.PostgresStore); !ok {
			t.Fatalf("got %T, want *profiles.PostgresStore", got)
		}
	})
	t.Run("cosmos backend returns the cosmos store", func(t *testing.T) {
		t.Parallel()
		got := chooseProfileStore(backendCosmos, pg, cosmos)
		if _, ok := got.(*profiles.CosmosStore); !ok {
			t.Fatalf("got %T, want *profiles.CosmosStore", got)
		}
	})
	t.Run("postgres backend with no pool yields a genuine nil interface", func(t *testing.T) {
		t.Parallel()
		if got := chooseProfileStore(backendPostgres, nil, cosmos); got != nil {
			t.Fatalf("got %T (non-nil interface), want genuine nil", got)
		}
	})
	t.Run("cosmos backend with no container yields a genuine nil interface", func(t *testing.T) {
		t.Parallel()
		if got := chooseProfileStore(backendCosmos, pg, nil); got != nil {
			t.Fatalf("got %T (non-nil interface), want genuine nil", got)
		}
	})
}

// TestChooseAdminStore mirrors TestChooseProfileStore for the admin profile
// store.
func TestChooseAdminStore(t *testing.T) {
	t.Parallel()

	cosmos := profiles.NewAdminStore(newFakeItems())
	pg := profiles.NewPostgresAdminStore(nil)

	t.Run("postgres backend returns the postgres admin store", func(t *testing.T) {
		t.Parallel()
		got := chooseAdminStore(backendPostgres, pg, cosmos)
		if _, ok := got.(*profiles.PostgresAdminStore); !ok {
			t.Fatalf("got %T, want *profiles.PostgresAdminStore", got)
		}
	})
	t.Run("cosmos backend returns the cosmos admin store", func(t *testing.T) {
		t.Parallel()
		got := chooseAdminStore(backendCosmos, pg, cosmos)
		if _, ok := got.(*profiles.AdminStore); !ok {
			t.Fatalf("got %T, want *profiles.AdminStore", got)
		}
	})
	t.Run("postgres backend with no pool yields a genuine nil interface", func(t *testing.T) {
		t.Parallel()
		if got := chooseAdminStore(backendPostgres, nil, cosmos); got != nil {
			t.Fatalf("got %T (non-nil interface), want genuine nil", got)
		}
	})
	t.Run("cosmos backend with no container yields a genuine nil interface", func(t *testing.T) {
		t.Parallel()
		if got := chooseAdminStore(backendCosmos, pg, nil); got != nil {
			t.Fatalf("got %T (non-nil interface), want genuine nil", got)
		}
	})
}

// TestChooseDeviceStore mirrors TestChooseAppStore for the device-token store.
func TestChooseDeviceStore(t *testing.T) {
	t.Parallel()

	cosmos := devicetokens.NewCosmosStore(newFakeItems())
	pg := devicetokens.NewPostgresStore(nil)

	t.Run("postgres backend returns the postgres store", func(t *testing.T) {
		t.Parallel()
		got := chooseDeviceStore(backendPostgres, pg, cosmos)
		if _, ok := got.(*devicetokens.PostgresStore); !ok {
			t.Fatalf("got %T, want *devicetokens.PostgresStore", got)
		}
	})
	t.Run("cosmos backend returns the cosmos store", func(t *testing.T) {
		t.Parallel()
		got := chooseDeviceStore(backendCosmos, pg, cosmos)
		if _, ok := got.(*devicetokens.CosmosStore); !ok {
			t.Fatalf("got %T, want *devicetokens.CosmosStore", got)
		}
	})
	t.Run("postgres backend with no pool yields a genuine nil interface", func(t *testing.T) {
		t.Parallel()
		if got := chooseDeviceStore(backendPostgres, nil, cosmos); got != nil {
			t.Fatalf("got %T (non-nil interface), want genuine nil", got)
		}
	})
	t.Run("cosmos backend with no container yields a genuine nil interface", func(t *testing.T) {
		t.Parallel()
		if got := chooseDeviceStore(backendCosmos, pg, nil); got != nil {
			t.Fatalf("got %T (non-nil interface), want genuine nil", got)
		}
	})
}

// TestChooseStateStore mirrors TestChooseAppStore for the notification-state
// store.
func TestChooseStateStore(t *testing.T) {
	t.Parallel()

	cosmos := notificationstate.NewCosmosStore(newFakeItems(), nil)
	pg := notificationstate.NewPostgresStore(nil)

	t.Run("postgres backend returns the postgres store", func(t *testing.T) {
		t.Parallel()
		got := chooseStateStore(backendPostgres, pg, cosmos)
		if _, ok := got.(*notificationstate.PostgresStore); !ok {
			t.Fatalf("got %T, want *notificationstate.PostgresStore", got)
		}
	})
	t.Run("cosmos backend returns the cosmos store", func(t *testing.T) {
		t.Parallel()
		got := chooseStateStore(backendCosmos, pg, cosmos)
		if _, ok := got.(*notificationstate.CosmosStore); !ok {
			t.Fatalf("got %T, want *notificationstate.CosmosStore", got)
		}
	})
	t.Run("postgres backend with no pool yields a genuine nil interface", func(t *testing.T) {
		t.Parallel()
		if got := chooseStateStore(backendPostgres, nil, cosmos); got != nil {
			t.Fatalf("got %T (non-nil interface), want genuine nil", got)
		}
	})
	t.Run("cosmos backend with no container yields a genuine nil interface", func(t *testing.T) {
		t.Parallel()
		if got := chooseStateStore(backendCosmos, pg, nil); got != nil {
			t.Fatalf("got %T (non-nil interface), want genuine nil", got)
		}
	})
}

// TestChooseNotifStore verifies backend selection for the notification store
// wired into NearbyRoutes (GetLatestUnreadByApplications path). The chooser
// returns a notifUnreadReader interface; type assertions confirm the backing
// concrete type.
func TestChooseNotifStore(t *testing.T) {
	t.Parallel()

	cosmos := notifications.NewCosmosStore(newFakeItems())
	pg := notifications.NewPostgresStore(nil)

	t.Run("postgres backend returns the postgres store", func(t *testing.T) {
		t.Parallel()
		got := chooseNotifStore(backendPostgres, pg, cosmos)
		if _, ok := got.(*notifications.PostgresStore); !ok {
			t.Fatalf("got %T, want *notifications.PostgresStore", got)
		}
	})
	t.Run("cosmos backend returns the cosmos store", func(t *testing.T) {
		t.Parallel()
		got := chooseNotifStore(backendCosmos, pg, cosmos)
		if _, ok := got.(*notifications.CosmosStore); !ok {
			t.Fatalf("got %T, want *notifications.CosmosStore", got)
		}
	})
	t.Run("postgres backend with no pool yields a genuine nil interface", func(t *testing.T) {
		t.Parallel()
		if got := chooseNotifStore(backendPostgres, nil, cosmos); got != nil {
			t.Fatalf("got %T (non-nil interface), want genuine nil", got)
		}
	})
	t.Run("cosmos backend with no container yields a genuine nil interface", func(t *testing.T) {
		t.Parallel()
		if got := chooseNotifStore(backendCosmos, pg, nil); got != nil {
			t.Fatalf("got %T (non-nil interface), want genuine nil", got)
		}
	})
}

// TestChooseSavedStore mirrors TestChooseAppStore for the saved-applications
// store.
func TestChooseSavedStore(t *testing.T) {
	t.Parallel()

	cosmos := savedapplications.NewCosmosStore(newFakeItems())
	pg := savedapplications.NewPostgresStore(nil)

	t.Run("postgres backend returns the postgres store", func(t *testing.T) {
		t.Parallel()
		got := chooseSavedStore(backendPostgres, pg, cosmos)
		if _, ok := got.(*savedapplications.PostgresStore); !ok {
			t.Fatalf("got %T, want *savedapplications.PostgresStore", got)
		}
	})
	t.Run("cosmos backend returns the cosmos store", func(t *testing.T) {
		t.Parallel()
		got := chooseSavedStore(backendCosmos, pg, cosmos)
		if _, ok := got.(*savedapplications.CosmosStore); !ok {
			t.Fatalf("got %T, want *savedapplications.CosmosStore", got)
		}
	})
	t.Run("postgres backend with no pool yields a genuine nil interface", func(t *testing.T) {
		t.Parallel()
		if got := chooseSavedStore(backendPostgres, nil, cosmos); got != nil {
			t.Fatalf("got %T (non-nil interface), want genuine nil", got)
		}
	})
	t.Run("cosmos backend with no container yields a genuine nil interface", func(t *testing.T) {
		t.Parallel()
		if got := chooseSavedStore(backendCosmos, pg, nil); got != nil {
			t.Fatalf("got %T (non-nil interface), want genuine nil", got)
		}
	})
}

// TestChooseOfferStore mirrors TestChooseAppStore for the offer-code store.
func TestChooseOfferStore(t *testing.T) {
	t.Parallel()

	cosmos := offercodes.NewCosmosStore(newFakeItems())
	pg := offercodes.NewPostgresStore(nil)

	t.Run("postgres backend returns the postgres store", func(t *testing.T) {
		t.Parallel()
		got := chooseOfferStore(backendPostgres, pg, cosmos)
		if _, ok := got.(*offercodes.PostgresStore); !ok {
			t.Fatalf("got %T, want *offercodes.PostgresStore", got)
		}
	})
	t.Run("cosmos backend returns the cosmos store", func(t *testing.T) {
		t.Parallel()
		got := chooseOfferStore(backendCosmos, pg, cosmos)
		if _, ok := got.(*offercodes.CosmosStore); !ok {
			t.Fatalf("got %T, want *offercodes.CosmosStore", got)
		}
	})
	t.Run("postgres backend with no pool yields a genuine nil interface", func(t *testing.T) {
		t.Parallel()
		if got := chooseOfferStore(backendPostgres, nil, cosmos); got != nil {
			t.Fatalf("got %T (non-nil interface), want genuine nil", got)
		}
	})
	t.Run("cosmos backend with no container yields a genuine nil interface", func(t *testing.T) {
		t.Parallel()
		if got := chooseOfferStore(backendCosmos, pg, nil); got != nil {
			t.Fatalf("got %T (non-nil interface), want genuine nil", got)
		}
	})
}

// TestChooseAppleNotifStore mirrors TestChooseAppStore for the Apple
// notification idempotency store.
func TestChooseAppleNotifStore(t *testing.T) {
	t.Parallel()

	cosmos := subscriptions.NewCosmosNotificationStore(newFakeItems(), time.Now)
	pg := subscriptions.NewPostgresNotificationStore(nil, time.Now)

	t.Run("postgres backend returns the postgres store", func(t *testing.T) {
		t.Parallel()
		got := chooseAppleNotifStore(backendPostgres, pg, cosmos)
		if _, ok := got.(*subscriptions.PostgresNotificationStore); !ok {
			t.Fatalf("got %T, want *subscriptions.PostgresNotificationStore", got)
		}
	})
	t.Run("cosmos backend returns the cosmos store", func(t *testing.T) {
		t.Parallel()
		got := chooseAppleNotifStore(backendCosmos, pg, cosmos)
		if _, ok := got.(*subscriptions.CosmosNotificationStore); !ok {
			t.Fatalf("got %T, want *subscriptions.CosmosNotificationStore", got)
		}
	})
	t.Run("postgres backend with no pool yields a genuine nil interface", func(t *testing.T) {
		t.Parallel()
		if got := chooseAppleNotifStore(backendPostgres, nil, cosmos); got != nil {
			t.Fatalf("got %T (non-nil interface), want genuine nil", got)
		}
	})
	t.Run("cosmos backend with no container yields a genuine nil interface", func(t *testing.T) {
		t.Parallel()
		if got := chooseAppleNotifStore(backendCosmos, pg, nil); got != nil {
			t.Fatalf("got %T (non-nil interface), want genuine nil", got)
		}
	})
}
