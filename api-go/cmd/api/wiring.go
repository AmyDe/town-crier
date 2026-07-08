package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/aasa"
	"github.com/AmyDe/town-crier/api-go/internal/admin"
	"github.com/AmyDe/town-crier/api-go/internal/api"
	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/assetlinks"
	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/authorities"
	"github.com/AmyDe/town-crier/api-go/internal/blobstore"
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
	"github.com/AmyDe/town-crier/api-go/internal/platform"
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
	"GET /v1/admin/stats":         {},
	"POST /v1/admin/offer-codes":  {},
	"GET /v1/admin/offer-codes":   {},
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
	// The anonymous application search (#821 Phase 3, tc-geq7h.3) reads only
	// public planning data (reference/address/description match, Postgres only —
	// PlanIt is never touched): a resident finding an application to share needs
	// no token, mirroring the by-slug read above.
	"GET /v1/applications/search": {},
	// The public share page is anonymous (public planning data only; no user data,
	// no cookies, no client-IP logging): a server-rendered HTML page for a single
	// application, the tracer surface of the shareable-page epic (#738).
	"GET /a/{authoritySlug}/{ref...}": {},
	// The public og:image card is anonymous (public planning data only; no user
	// data/cookies/IP-logging): a baked OSM map of the site served as the share
	// page's unfurl image (#738 Slice 2). The ".png" suffix is enforced in the
	// handler, so the registered pattern is suffix-free.
	"GET /og/{authoritySlug}/{ref...}": {},
	// The Apple App Site Association document is anonymous: Apple's daemon fetches
	// it without a bearer token to associate the share host with the iOS app for
	// Universal Links (#738 Slice 3). Content-Type is application/json and the path
	// carries no ".json" extension.
	"GET /.well-known/apple-app-site-association": {},
	// The Android Digital Asset Links document is anonymous: Android's package
	// manager fetches it without a bearer token to verify the app's autoVerify
	// intent filter for App Links (GH#782), mirroring the AASA entry above.
	"GET /.well-known/assetlinks.json": {},
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
// router wires into three places: the latest-unread lookup for NearbyRoutes, the
// batched per-user tally for the admin user list, and the whole-table totals for
// the admin stats reach block. *notifications.PostgresStore (which satisfies the
// full notifications.Store) satisfies it structurally.
type notifStoreReader interface {
	GetLatestUnreadByApplications(ctx context.Context, userID string, applicationUIDs []string) (map[string]notifications.LatestUnread, error)
	CountsByUsers(ctx context.Context, userIDs []string) (map[string]notifications.NotificationCounts, error)
	Totals(ctx context.Context) (notifications.NotificationTotals, error)
}

// savedStoreReader is the saved-application store the router wires into the
// feature routes plus the admin surface. It embeds the full savedapplications
// Store (feature routes) and adds the admin-read batched count + global total.
// *savedapplications.PostgresStore satisfies it.
type savedStoreReader interface {
	savedapplications.Store
	CountsByUsers(ctx context.Context, userIDs []string) (map[string]int, error)
	Count(ctx context.Context) (int, error)
}

// deviceStoreReader is the device-registration store the router wires into the
// device routes plus the admin surface. It embeds the full devicetokens Store
// and adds the admin-read batched count + global total.
// *devicetokens.PostgresStore satisfies it.
type deviceStoreReader interface {
	devicetokens.Store
	CountsByUsers(ctx context.Context, userIDs []string) (map[string]int, error)
	Count(ctx context.Context) (int, error)
}

// offerStoreReader is the offer-code store the router wires into the redeem
// routes plus the admin surface. It embeds the full offercodes Store and adds
// the admin-read batched redemption lookup plus the admin code listing.
// *offercodes.PostgresStore satisfies it.
type offerStoreReader interface {
	offercodes.Store
	RedeemedByUsers(ctx context.Context, userIDs []string) (map[string][]offercodes.RedeemedOfferCode, error)
	List(ctx context.Context, labelFilter *string, limit int) ([]offercodes.ListedOfferCode, error)
}

// adminUserStore is the admin profile store the router wires into admin.Routes:
// the full AdminProfileStore plus the two stats aggregates (paid-tier candidates
// and the whole-base UserStats). *profiles.PostgresAdminStore satisfies it.
type adminUserStore interface {
	profiles.AdminProfileStore
	PaidCandidates(ctx context.Context) ([]*profiles.UserProfile, error)
	UserStats(ctx context.Context, now time.Time) (profiles.UserStats, error)
}

// newRouter wires the feature routes onto a mux and wraps it in the production
// middleware chain. Ordering, from outermost to innermost:
//
//		CORS -> SecurityHeaders -> ErrorBody -> Recover -> RequireAuth -> AnonRateLimit -> RateLimit -> RecordActivity -> mux
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
//	    dispatches through AnonRateLimit, rate limiting and activity recording.
//	  - AnonRateLimit (GH#868 Phase 1) is a no-op for authenticated requests (it
//	    only meters when auth.Subject is empty) and, unlike RateLimit/
//	    RecordActivity below, it always wraps dispatch regardless of whether the
//	    profile store is wired — a store-less boot must not leave anonymous
//	    routes completely unmetered. GET /health and GET /v1/health are exempt
//	    (ACA probes; see middleware.AnonRateLimit's doc for the convention).
//	  - RateLimit/RecordActivity are no-ops for anonymous routes (no subject),
//	    the mirror image of AnonRateLimit's no-op for authenticated ones.
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
	deviceStore deviceStoreReader,
	stateStore notificationstate.Store,
	notifStore notifStoreReader,
	watchZoneStore watchzones.Store,
	appStore applications.Store,
	savedStore savedStoreReader,
	geocodeClient *geocoding.Client,
	designationClient *designations.Client,
	offerStore offerStoreReader,
	adminStore adminUserStore,
	adminKey string,
	siteBuildKey string,
	jwsVerifier *subscriptions.JWSVerifier,
	appleNotifStore subscriptions.Store,
	appleBundleID string,
	appleEnvironments []string,
	registry *metrics.Registry,
	shareCardCache *blobstore.Store,
	anonRateLimitRequests int,
	anonRateLimitWindowSeconds int,
	logger *slog.Logger,
) http.Handler {
	mux := http.NewServeMux()
	health.Routes(mux, logger)
	versionconfig.Routes(mux, logger)
	legal.Routes(mux, logger)
	authorities.Routes(mux, logger)
	api.Routes(mux, logger)
	// The Apple App Site Association document (#738 Slice 3) is stateless and needs
	// no store, so it is registered unconditionally on the share host. The App ID
	// is composed from the canonical team + bundle constants (fixed contract), not
	// runtime config.
	aasa.Routes(mux, platform.AppleUniversalLinkAppID(), logger)
	// The Android Digital Asset Links document (GH#782) is stateless and needs
	// no store, so it is registered unconditionally on the share host, mirroring
	// AASA above. The package/fingerprint contract is composed from fixed
	// constants (not runtime config): see assetlinks.DefaultPackages.
	assetlinks.Routes(mux, assetlinks.DefaultPackages(), logger)
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
	// AnonRateLimit (GH#868 Phase 1) always wraps whatever dispatch chain was
	// built above — unlike RateLimit/RecordActivity, it needs no profile store,
	// so it must not be skipped on a store-less boot: that would leave every
	// anonymous route (a scraping target for a future public geo endpoint, and
	// load that ultimately lands on PlanIt) completely unmetered. It is a no-op
	// for authenticated requests, so it never interferes with the per-subject
	// RateLimit it wraps.
	_ = anonRateLimitWindowSeconds
	_ = anonRateLimitRequests
	// TEMP-RED-CHECK: AnonRateLimit deliberately not wired yet.
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
		// The anonymous application search (#821 Phase 3, tc-geq7h.3) reads from the
		// same store; it needs no build key (unlike the two SEO routes above) since
		// it is not a build-time-only surface — it is the public-facing endpoint the
		// /search web page (tc-geq7h.4) calls directly from a visitor's browser. It is
		// anonymous to Auth0 (see anonymousPatterns) and, like every other anonymous
		// route, metered by the per-IP AnonRateLimit (GH#868 Phase 1, above): the
		// per-subject RateLimit no-ops on a request with no subject, but
		// AnonRateLimit does not — it is the scheme that actually covers this route.
		applications.SearchRoutes(mux, appStore, authorities.NewLookup(), logger)
		// The public share page (#738): an anonymous, server-rendered HTML page for a
		// single application at GET /a/{authoritySlug}/{ref...}. It point-reads the
		// same applications store and resolves the authority slug via the static
		// authority lookup; it reads no user data and emits only public planning data.
		sharepage.Routes(mux, appStore, authorities.NewLookup(), logger)
		// The og:image map card (#738 Slice 2/3): an anonymous GET /og/{slug}/{ref}.png
		// serving a baked OSM map of the site as the share page's unfurl image. It
		// reuses the same applications store + authority lookup and the real OSM tile
		// client. The cache is the Azure share-cards Blob container (ADR 0037); it is
		// optional. When SHARE_CARDS_BLOB_URL is unset shareCardCache is a nil pointer,
		// and we MUST pass a genuine nil interface (not the typed-nil *blobstore.Store)
		// so the handler's cache==nil check disables caching and regenerates on demand.
		if shareCardCache != nil {
			sharepage.ImageRoutes(mux, appStore, authorities.NewLookup(), sharepage.NewOSMTileClient(), shareCardCache, logger)
		} else {
			sharepage.ImageRoutes(mux, appStore, authorities.NewLookup(), sharepage.NewOSMTileClient(), nil, logger)
		}
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
		// cross-user admin store backs grant/list/stats; the offer-code store backs
		// both generate (writer) and the active-code column + comped-classification
		// reader (RedeemedByUsers). The notif/saved/device readers enrich the user
		// list and feed the stats reach block; each may be nil on a store-less boot,
		// and the handlers treat a nil reader as "metric absent".
		admin.Routes(mux, adminKey, adminStore, notifStore, savedStore, deviceStore, offerStore, auth0, offerStore, offercodes.NewRandomGenerator(), time.Now, logger)
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
