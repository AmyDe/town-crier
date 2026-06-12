package main

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/api"
	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/authorities"
	"github.com/AmyDe/town-crier/api-go/internal/health"
	"github.com/AmyDe/town-crier/api-go/internal/legal"
	"github.com/AmyDe/town-crier/api-go/internal/middleware"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/versionconfig"
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
// profile-backed rate-limit/activity middlewares are skipped entirely.
func newRouter(validator auth.TokenValidator, corsOrigins []string, store *profiles.CosmosStore, auth0 profiles.Auth0Manager, proDomains string, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	health.Routes(mux, logger)
	versionconfig.Routes(mux, logger)
	legal.Routes(mux, logger)
	authorities.Routes(mux, logger)
	api.Routes(mux, logger)

	var dispatch http.Handler = mux
	if store != nil {
		profiles.Routes(mux, store, auth0, proDomains, time.Now, logger)
		dispatch = middleware.RateLimit(middleware.NewRateLimitStore(), profiles.NewTierLookup(store), logger)(
			middleware.RecordActivity(profiles.NewActivityRecorder(store), time.Now, logger)(mux),
		)
	}

	authed := auth.RequireAuth(validator, &dispatchMux{ServeMux: mux, dispatch: dispatch}, anonymousPatterns)
	return middleware.CORS(corsOrigins)(
		middleware.ErrorBody(logger)(
			middleware.Recover(logger)(authed),
		),
	)
}
