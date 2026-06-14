package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/admin"
	"github.com/AmyDe/town-crier/api-go/internal/api"
	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/authorities"
	"github.com/AmyDe/town-crier/api-go/internal/demoaccount"
	"github.com/AmyDe/town-crier/api-go/internal/designations"
	"github.com/AmyDe/town-crier/api-go/internal/devicetokens"
	"github.com/AmyDe/town-crier/api-go/internal/geocoding"
	"github.com/AmyDe/town-crier/api-go/internal/health"
	"github.com/AmyDe/town-crier/api-go/internal/legal"
	"github.com/AmyDe/town-crier/api-go/internal/middleware"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/notificationstate"
	"github.com/AmyDe/town-crier/api-go/internal/offercodes"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/savedapplications"
	"github.com/AmyDe/town-crier/api-go/internal/subscriptions"
	"github.com/AmyDe/town-crier/api-go/internal/versionconfig"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// anonymousPatterns lists the mux patterns registered with AllowAnonymous in
// the .NET API (GH#418). Every other matched route — and every unmatched
// request — requires a valid Auth0 bearer token via the fallback-deny policy.
// Keep this in lockstep with the routes wired below.
var anonymousPatterns = map[string]struct{}{
	"GET /health":                  {},
	"GET /v1/health":               {},
	"GET /v1/version-config":       {},
	"GET /v1/legal/{documentType}": {},
	"GET /v1/authorities":          {},
	"GET /v1/authorities/{$}":      {},
	"GET /v1/authorities/{id}":     {},
	// The demo-account endpoint is anonymous so Apple's App Store reviewer can
	// reach a fully-provisioned Pro account without a token, matching .NET's
	// AllowAnonymous.
	"GET /v1/demo-account": {},
	// Admin routes are anonymous to Auth0 (no bearer token); they are
	// authenticated solely by the X-Admin-Key gate inside the handlers, matching
	// .NET's AllowAnonymous + AdminApiKeyFilter.
	"PUT /v1/admin/subscriptions": {},
	"GET /v1/admin/users":         {},
	"POST /v1/admin/offer-codes":  {},
	// The App Store Server Notifications webhook is Apple -> API, not user-facing,
	// so it is anonymous to Auth0; the signed JWS is its authentication. (The
	// sibling POST /v1/subscriptions/verify is authed and absent here.)
	"POST /v1/webhooks/appstore": {},
}

// dispatchMux satisfies auth.RequireAuth's routeMatcher: pattern matching comes
// from the embedded mux, while dispatch runs the post-auth pipeline (rate limit
// -> activity recording -> handlers). This reproduces .NET's middleware order
// — UseAuthorization, RateLimitMiddleware, RecordUserActivityMiddleware — in a
// world where Go's mux both matches and dispatches.
type dispatchMux struct {
	*http.ServeMux
	dispatch http.Handler
}

func (d *dispatchMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	d.dispatch.ServeHTTP(w, r)
}

// notificationStateDeleter bridges the notification-state store's DeleteByUserID
// (one watermark document per user) to the profiles cascade's ChildDeleter
// contract (DeleteAllByUserID). The other four cascade stores already expose
// DeleteAllByUserID and satisfy the interface directly.
type notificationStateDeleter struct {
	s *notificationstate.CosmosStore
}

func (d notificationStateDeleter) DeleteAllByUserID(ctx context.Context, userID string) error {
	return d.s.DeleteByUserID(ctx, userID)
}

// newRouter wires the feature routes onto a mux and wraps it in the production
// middleware chain. Ordering, from outermost to innermost, mirrors the .NET
// pipeline (WebApplicationExtensions.UseMiddlewarePipeline):
//
//		CORS -> ErrorBody -> Recover -> RequireAuth -> RateLimit -> RecordActivity -> mux
//
//	  - CORS runs first so its headers are present on every response, including
//	    the 401s and 500s produced further in.
//	  - ErrorBody backfills the PascalCase envelope onto any bodyless >= 400.
//	  - Recover converts a handler panic into a 500 + Detail, which ErrorBody
//	    then renders — the Go equivalent of ErrorResponseMiddleware's try/catch.
//	  - RequireAuth applies the Auth0 bearer + fallback-deny policy, then
//	    dispatches through rate limiting and activity recording (both no-ops for
//	    anonymous routes, which carry no subject).
//
// store is nil when Cosmos is not configured (local boot without env): the
// /v1/me routes are then unwired — requests fall to the 401 fallback — and the
// profile-backed rate-limit/activity middlewares are skipped entirely. The
// device-token, notification-state and watch-zone stores follow the same
// nil-means-unwired convention. (The watch-zone preferences endpoints are
// served by profiles.Routes off the profile store, so they come up with the
// /v1/me routes, not the watch-zone store.)
func newRouter(validator auth.TokenValidator, corsOrigins []string, store *profiles.CosmosStore, auth0 profiles.Auth0Manager, proDomains string, cascade profiles.CascadeDeleters, deviceStore *devicetokens.CosmosStore, stateStore *notificationstate.CosmosStore, notifStore *notifications.CosmosStore, watchZoneStore *watchzones.CosmosStore, appStore *applications.CosmosStore, savedStore *savedapplications.CosmosStore, geocodeClient *geocoding.Client, designationClient *designations.Client, offerStore *offercodes.CosmosStore, adminStore *profiles.AdminStore, adminKey string, jwsVerifier *subscriptions.JWSVerifier, appleNotifStore *subscriptions.CosmosNotificationStore, appleBundleID string, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	health.Routes(mux, logger)
	versionconfig.Routes(mux, logger)
	legal.Routes(mux, logger)
	authorities.Routes(mux, logger)
	api.Routes(mux, logger)
	// Geocode and designations are authed (absent from anonymousPatterns) and
	// have no Cosmos dependency — they call outbound UK services — so they are
	// always wired, even on a Cosmos-less local boot.
	geocoding.Routes(mux, geocodeClient, logger)
	designations.Routes(mux, designationClient, logger)

	var dispatch http.Handler = mux
	if store != nil {
		profiles.Routes(mux, store, auth0, proDomains, cascade, time.Now, logger)
		dispatch = middleware.RateLimit(middleware.NewRateLimitStore(), profiles.NewTierLookup(store), logger)(
			middleware.RecordActivity(profiles.NewActivityRecorder(store), time.Now, logger)(mux),
		)
	}
	if deviceStore != nil {
		devicetokens.Routes(mux, deviceStore, time.Now, logger)
	}
	if stateStore != nil {
		notificationstate.Routes(mux, stateStore, time.Now, logger)
	}
	if watchZoneStore != nil {
		watchzones.Routes(mux, watchZoneStore, logger)
		// application-authorities derives from the user's watch zones plus the
		// static authority data; it needs no Cosmos applications store.
		watchzones.AuthoritiesRoutes(mux, watchZoneStore, authorities.NewLookup(), logger)
	}
	if appStore != nil {
		// Refresh-on-tap heals a saved row's snapshot when the user views an app
		// they've saved; it needs the saved store. Pass a genuine nil interface
		// (not a typed-nil pointer) when saving isn't wired, so the handler's
		// nil-check disables the side effect cleanly.
		if savedStore != nil {
			applications.Routes(mux, appStore, savedapplications.NewSnapshotRefresher(savedStore, time.Now), logger)
		} else {
			applications.Routes(mux, appStore, nil, logger)
		}
	}
	if store != nil && watchZoneStore != nil && appStore != nil && stateStore != nil && notifStore != nil {
		// Watch-zone create (returns nearby applications) + the per-zone
		// applications list. Create enforces the tier's zone quota from the
		// profile store and resolves the authority from coordinates via the
		// geocode client when the request omits one; the applications list
		// augments each row with its latest unread notification.
		watchzones.NearbyRoutes(mux, watchZoneStore, store, geocodeClient, appStore, stateStore, notifStore, uuid.NewString, time.Now, logger)
	}
	if store != nil && watchZoneStore != nil && appStore != nil {
		// Demo account (anonymous): seeds a Pro profile, a Westminster watch zone,
		// and five fixed applications on first call, then returns the zone plus
		// its nearby applications. Needs the profile, watch-zone and application
		// stores; the spatial lookup hits the applications store's FindNearby.
		demoaccount.Routes(mux, store, watchZoneStore, appStore, time.Now, logger)
	}
	if savedStore != nil && appStore != nil {
		// The save path dual-writes: the master application record (appStore) then
		// the saved row, so both stores are required to wire the endpoints.
		savedapplications.Routes(mux, savedStore, appStore, time.Now, logger)
	}
	if store != nil && offerStore != nil {
		// Offer-code redeem is authed: it loads the caller's profile, grants the
		// coded tier, and syncs Auth0. Needs both the profile and offer-code stores.
		offercodes.Routes(mux, offerStore, store, auth0, time.Now, logger)
	}
	if adminStore != nil && offerStore != nil {
		// Admin endpoints are anonymous to Auth0 and gated by the X-Admin-Key. The
		// cross-partition admin store backs grant/list; the offer-code store backs
		// generate.
		admin.Routes(mux, adminKey, adminStore, auth0, offerStore, offercodes.NewRandomGenerator(), time.Now, logger)
	}
	if store != nil && adminStore != nil && jwsVerifier != nil && appleNotifStore != nil {
		// Subscriptions: verify (authed, by user id via the profile store) and the
		// App Store webhook (anonymous, by original transaction id via the admin
		// store, deduped through the AppleNotifications idempotency store).
		subscriptions.Routes(mux, jwsVerifier, store, adminStore, auth0, appleNotifStore, appleBundleID, time.Now, logger)
	}

	authed := auth.RequireAuth(validator, &dispatchMux{ServeMux: mux, dispatch: dispatch}, anonymousPatterns)
	chain := middleware.CORS(corsOrigins)(
		middleware.ErrorBody(logger)(
			middleware.Recover(logger)(authed),
		),
	)
	// otelhttp is the outermost wrapper so its span covers the whole request,
	// including the CORS, error-envelope and panic-recovery middleware. When
	// telemetry is disabled (no OTLP endpoint) the global no-op TracerProvider
	// makes this produce no-op spans at negligible cost, so the wiring stays
	// unconditional and every existing httptest assertion is unaffected.
	return otelhttp.NewHandler(chain, "town-crier-api-go")
}
