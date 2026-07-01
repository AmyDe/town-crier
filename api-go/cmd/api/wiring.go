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
	"github.com/AmyDe/town-crier/api-go/internal/metrics"
	"github.com/AmyDe/town-crier/api-go/internal/middleware"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/notificationstate"
	"github.com/AmyDe/town-crier/api-go/internal/offercodes"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/savedapplications"
	"github.com/AmyDe/town-crier/api-go/internal/sharepage"
	"github.com/AmyDe/town-crier/api-go/internal/subscriptions"
	"github.com/AmyDe/town-crier/api-go/internal/versionconfig"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// anonymousPatterns lists the mux patterns that are served without a token.
// Every other matched route — and every unmatched request — requires a valid
// Auth0 bearer token via the fallback-deny policy.
// Keep this in lockstep with the routes wired below.
var anonymousPatterns = map[string]struct{}{
	"GET /health":                  {},
	"GET /v1/health":               {},
	"GET /v1/version-config":       {},
	"GET /v1/legal/{documentType}": {},
	// The demo-account endpoint is anonymous so Apple's App Store reviewer can
	// reach a fully-provisioned Pro account without a token (no bearer required).
	"GET /v1/demo-account": {},
	// Admin routes are anonymous to Auth0 (no bearer token); they are
	// authenticated solely by the X-Admin-Key gate inside the handlers.
	"PUT /v1/admin/subscriptions": {},
	"GET /v1/admin/users":         {},
	"POST /v1/admin/offer-codes":  {},
	// The App Store Server Notifications webhook is Apple -> API, not user-facing,
	// so it is anonymous to Auth0; the signed JWS is its authentication. (The
	// sibling POST /v1/subscriptions/verify is authed and absent here.)
	"POST /v1/webhooks/appstore": {},
	// The build-time SEO endpoints are anonymous to Auth0 (no bearer token); they
	// are authenticated solely by the X-Build-Key gate inside the handler, mirroring
	// the admin routes. They read only public planning data. The first feeds
	// authority pages; the second feeds town pages (bounded geo query).
	"GET /v1/authorities/{id}/applications": {},
	"GET /v1/applications/near":             {},
	// The by-slug application read is anonymous (public planning data only, no
	// user/subscriber data, no refresh-on-tap): it resolves an authority slug to
	// its area id and point-reads the application, feeding the public share page
	// and iOS inbound deep-link resolution (#738). The sibling by-id read stays
	// authed (absent from this map).
	"GET /v1/applications/by-slug/{authoritySlug}/{ref...}": {},
	// The public share page is anonymous (public planning data only; no user data,
	// no cookies, no client-IP logging): a server-rendered HTML page for a single
	// application, the tracer surface of the shareable-page epic (#738).
	"GET /a/{authoritySlug}/{ref...}": {},
}

// dispatchMux satisfies auth.RequireAuth's routeMatcher: pattern matching comes
// from the embedded mux, while dispatch runs the post-auth pipeline (rate limit
// -> activity recording -> handlers). This encodes the pipeline order in a
// world where Go's mux both matches and dispatches.
type dispatchMux struct {
	*http.ServeMux
	dispatch http.Handler
}

func (d *dispatchMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	d.dispatch.ServeHTTP(w, r)
}

// notifStoreReader is the consumer-side slice of the notification store the
// router wires into two places: the latest-unread lookup for NearbyRoutes and
// the batched per-user tally for the admin user list. *notifications.PostgresStore
// (which satisfies the full notifications.Store) satisfies it structurally.
type notifStoreReader interface {
	GetLatestUnreadByApplications(ctx context.Context, userID string, applicationUIDs []string) (map[string]notifications.LatestUnread, error)
	CountsByUsers(ctx context.Context, userIDs []string) (map[string]notifications.NotificationCounts, error)
}

// newRouter wires the feature routes onto a mux and wraps it in the production
// middleware chain. Ordering, from outermost to innermost:
//
//		CORS -> SecurityHeaders -> ErrorBody -> Recover -> RequireAuth -> RateLimit -> RecordActivity -> mux
//
//	  - CORS runs first so its headers are present on every response, including
//	    the 401s and 500s produced further in.
//	  - SecurityHeaders sits just inside CORS so X-Content-Type-Options: nosniff
//	    is set on every response, including the error-body envelopes and
//	    panic-recovery 500s produced further in (GH#521).
//	  - ErrorBody backfills the PascalCase envelope onto any bodyless >= 400.
//	  - Recover converts a handler panic into a 500 + Detail, which ErrorBody
//	    then renders — the Go equivalent of ErrorResponseMiddleware's try/catch.
//	  - RequireAuth applies the Auth0 bearer + fallback-deny policy, then
//	    dispatches through rate limiting and activity recording (both no-ops for
//	    anonymous routes, which carry no subject).
//
// The store parameters are consumer-side interfaces backed by the Postgres
// stores. Each retains the nil-means-unwired convention so a store-less local
// boot (no datastore) leaves the corresponding routes unwired — requests fall to
// the 401 fallback — and the profile-backed rate-limit/activity middlewares are
// skipped entirely. (The watch-zone preferences endpoints are served by
// profiles.Routes off the profile store, so they come up with the /v1/me routes,
// not the watch-zone store.)
func newRouter(
	validator auth.TokenValidator,
	corsOrigins []string,
	store profiles.Store,
	auth0 profiles.Auth0Manager,
	cascade profiles.CascadeDeleters,
	exportReaders profiles.ExportReaders,
	deviceStore devicetokens.Store,
	stateStore notificationstate.Store,
	notifStore notifStoreReader,
	watchZoneStore watchzones.Store,
	appStore applications.Store,
	savedStore savedapplications.Store,
	geocodeClient *geocoding.Client,
	designationClient *designations.Client,
	offerStore offercodes.Store,
	adminStore profiles.AdminProfileStore,
	adminKey string,
	siteBuildKey string,
	jwsVerifier *subscriptions.JWSVerifier,
	appleNotifStore subscriptions.Store,
	appleBundleID string,
	appleEnvironments []string,
	registry *metrics.Registry,
	logger *slog.Logger,
) http.Handler {
	mux := http.NewServeMux()
	health.Routes(mux, logger)
	versionconfig.Routes(mux, logger)
	legal.Routes(mux, logger)
	authorities.Routes(mux, logger)
	api.Routes(mux, logger)
	// Geocode and designations are authed (absent from anonymousPatterns) and
	// have no store dependency — they call outbound UK services — so they are
	// always wired, even on a store-less local boot.
	geocoding.Routes(mux, geocodeClient, logger)
	designations.Routes(mux, designationClient, logger)

	var dispatch http.Handler = mux
	if store != nil {
		profiles.Routes(mux, store, auth0, cascade, exportReaders, time.Now, logger)
		dispatch = middleware.RateLimit(middleware.NewRateLimitStore(), profiles.NewTierLookup(store, time.Now), logger)(
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
		// The delete path decrements the watch-zone quota counter on the profile
		// via CAS, so it needs the CAS-capable profile store. Only pass the option
		// when the profile store is genuinely present — passing a typed-nil
		// *profiles.PostgresStore would wrap into a non-nil interface and defeat the
		// handler's nil-check (the same typed-nil gotcha guarded for the saved
		// store above).
		zoneOpts := []watchzones.Option{watchzones.WithMetricsRecorder(registry)}
		if store != nil {
			zoneOpts = append(zoneOpts, watchzones.WithProfileCAS(store))
		}
		watchzones.Routes(mux, watchZoneStore, logger, zoneOpts...)
		// application-authorities derives from the user's watch zones plus the
		// static authority data; it needs no applications store.
		watchzones.AuthoritiesRoutes(mux, watchZoneStore, authorities.NewLookup(), logger)
	}
	if appStore != nil {
		// Refresh-on-tap heals a saved row's snapshot when the user views an app
		// they've saved; it needs the saved store. Pass a genuine nil interface
		// (not a typed-nil pointer) when saving isn't wired, so the handler's
		// nil-check disables the side effect cleanly.
		if savedStore != nil {
			applications.Routes(mux, appStore, savedapplications.NewSnapshotRefresher(savedStore, time.Now), authorities.NewLookup(), logger)
		} else {
			applications.Routes(mux, appStore, nil, authorities.NewLookup(), logger)
		}
		// The build-time SEO endpoints (anonymous to Auth0, gated by the dedicated
		// X-Build-Key) read recent applications from the same store: RecentRoutes by
		// authority (authority pages) and NearRoutes by a bounded geo point (town
		// pages). Both are registered here because they need the applications store;
		// the routes are absent on a store-less local boot, where they fall to the
		// 401 fallback.
		applications.RecentRoutes(mux, appStore, siteBuildKey, logger)
		applications.NearRoutes(mux, appStore, siteBuildKey, logger)
		// The public share page (#738): an anonymous, server-rendered HTML page for a
		// single application at GET /a/{authoritySlug}/{ref...}. It point-reads the
		// same applications store and resolves the authority slug via the static
		// authority lookup; it reads no user data and emits only public planning data.
		sharepage.Routes(mux, appStore, authorities.NewLookup(), logger)
	}
	if store != nil && watchZoneStore != nil && appStore != nil && notifStore != nil {
		// Watch-zone create (returns nearby applications) + the per-zone
		// applications list. Create enforces the tier's zone quota atomically via
		// CAS on the profile counter (WithProfileCAS) — this is the only create
		// path, so concurrent creates can never exceed the tier limit (F5/#515).
		// store is non-nil here (guarded above), so no typed-nil concern. It
		// resolves the authority from coordinates via the geocode client when the
		// request omits one; the applications list augments each row with its
		// latest unread notification (read_at IS NULL, ADR 0035).
		watchzones.NearbyRoutes(mux, watchZoneStore, store, geocodeClient, appStore, notifStore, uuid.NewString, time.Now, logger, watchzones.WithMetricsRecorder(registry), watchzones.WithProfileCAS(store))
	}
	if store != nil && watchZoneStore != nil && appStore != nil {
		// Demo account (anonymous): seeds a Pro profile, a Westminster watch zone,
		// and five fixed applications on first call, then returns the zone plus
		// its nearby applications. Needs the profile, watch-zone and application
		// stores; the spatial lookup hits the applications store's FindNearby.
		demoaccount.Routes(mux, store, watchZoneStore, appStore, time.Now, logger)
	}
	if savedStore != nil && appStore != nil {
		// The save path looks up the master application record (appStore) to verify
		// it exists, then writes only the per-user saved row. Both stores are
		// required to wire the endpoints.
		savedapplications.Routes(mux, savedStore, appStore, time.Now, logger)
	}
	if store != nil && offerStore != nil {
		// Offer-code redeem is authed: it loads the caller's profile, grants the
		// coded tier, and syncs Auth0. Needs both the profile and offer-code stores.
		offercodes.Routes(mux, offerStore, store, auth0, time.Now, logger)
	}
	if adminStore != nil && offerStore != nil {
		// Admin endpoints are anonymous to Auth0 and gated by the X-Admin-Key. The
		// cross-user admin store backs grant/list; the offer-code store backs
		// generate.
		admin.Routes(mux, adminKey, adminStore, notifStore, auth0, offerStore, offercodes.NewRandomGenerator(), time.Now, logger)
	}
	if store != nil && adminStore != nil && jwsVerifier != nil && appleNotifStore != nil {
		// Subscriptions: verify (authed, by user id via the profile store) and the
		// App Store webhook (anonymous, by original transaction id via the admin
		// store, deduped through the AppleNotifications idempotency store).
		subscriptions.Routes(mux, jwsVerifier, store, adminStore, auth0, appleNotifStore, appleBundleID, appleEnvironments, time.Now, logger)
	}

	authed := auth.RequireAuth(validator, &dispatchMux{ServeMux: mux, dispatch: dispatch}, anonymousPatterns)
	chain := middleware.CORS(corsOrigins)(
		middleware.SecurityHeaders(
			middleware.ErrorBody(logger)(
				middleware.Recover(logger)(authed),
			),
		),
	)
	// RouteSpan names the request span after the matched route and records the
	// HTTP status code on it (tc-r8eo). It must run inside the otelhttp span — so
	// the span it decorates is the request span — yet outermost among our own
	// middleware so it observes the final status. It resolves the pattern from the
	// mux directly (the same lookup RequireAuth uses), which is why it takes mux
	// rather than relying on r.Pattern (lost across the chain's context copies).
	chain = middleware.RouteSpan(mux)(chain)
	// otelhttp is the outermost wrapper so its span covers the whole request,
	// including the CORS, error-envelope and panic-recovery middleware. When
	// telemetry is disabled (no OTLP endpoint) the global no-op TracerProvider
	// makes this produce no-op spans at negligible cost, so the wiring stays
	// unconditional and every existing httptest assertion is unaffected.
	return otelhttp.NewHandler(chain, "town-crier-api-go")
}
