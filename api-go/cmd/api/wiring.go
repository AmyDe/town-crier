package main

import (
	"log/slog"
	"net/http"

	"github.com/AmyDe/town-crier/api-go/internal/api"
	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/authorities"
	"github.com/AmyDe/town-crier/api-go/internal/health"
	"github.com/AmyDe/town-crier/api-go/internal/legal"
	"github.com/AmyDe/town-crier/api-go/internal/middleware"
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

// newRouter wires the feature routes onto a mux and wraps it in the production
// middleware chain. Ordering, from outermost to innermost, mirrors the .NET
// pipeline (Program.cs / WebApplicationExtensions):
//
//		CORS -> ErrorBody -> Recover -> RequireAuth(mux)
//
//	  - CORS runs first so its headers are present on every response, including
//	    the 401s and 500s produced further in.
//	  - ErrorBody backfills the PascalCase envelope onto any bodyless >= 400.
//	  - Recover converts a handler panic into a 500 + Detail, which ErrorBody
//	    then renders — the Go equivalent of ErrorResponseMiddleware's try/catch.
//	  - RequireAuth applies the Auth0 bearer + fallback-deny policy, dispatching
//	    matched, authorised requests to the mux.
func newRouter(validator auth.TokenValidator, corsOrigins []string, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	health.Routes(mux, logger)
	versionconfig.Routes(mux, logger)
	legal.Routes(mux, logger)
	authorities.Routes(mux, logger)
	api.Routes(mux, logger)

	authed := auth.RequireAuth(validator, mux, anonymousPatterns)
	return middleware.CORS(corsOrigins)(
		middleware.ErrorBody(logger)(
			middleware.Recover(logger)(authed),
		),
	)
}
