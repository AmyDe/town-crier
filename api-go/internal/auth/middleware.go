// Package auth replicates the .NET API's Auth0 JWT bearer authentication and
// fallback-deny authorization (GH#418, parity behaviours 6 and the JwtBearer
// mapping). Every route requires a valid Auth0 access token unless it was
// explicitly registered as anonymous; unmatched routes fall through to the same
// 401 challenge, mirroring ASP.NET's "no endpoint -> fallback policy" flow.
package auth

import (
	"context"
	"net/http"
	"strings"
)

// TokenValidator validates a raw bearer token and returns its subject (the JWT
// `sub` claim). The concrete *Auth0Validator satisfies it; unit tests substitute
// a hand-written fake so no JWKS network call happens. It is exported because
// the binary's wiring (cmd/api) names it when assembling the router.
type TokenValidator interface {
	ValidateToken(ctx context.Context, token string) (string, error)
}

// routeMatcher is the subset of *http.ServeMux the middleware uses: it both
// reports which registered pattern (if any) a request matches — the Go
// equivalent of ASP.NET resolving an endpoint before authorization — and
// dispatches the request once the auth decision is made.
type routeMatcher interface {
	http.Handler
	Handler(r *http.Request) (h http.Handler, pattern string)
}

type subjectKey struct{}

// RequireAuth wraps mux with Auth0 bearer authentication and fallback-deny
// authorization. anonymousPatterns is the set of mux patterns (e.g.
// "GET /v1/health") registered with AllowAnonymous in .NET; requests matching
// one are served without a token. Every other matched route requires a valid
// bearer token, and any unmatched request is denied with the same 401 challenge
// — reproducing .NET's behaviour where an unselected endpoint triggers the
// RequireAuthenticatedUser fallback policy.
func RequireAuth(v TokenValidator, mux routeMatcher, anonymousPatterns map[string]struct{}) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, pattern := mux.Handler(r)

		// No pattern matched (unknown path, wrong method, or a constrained route
		// that did not match): deny, exactly as .NET's fallback policy does when
		// no endpoint is selected.
		if pattern == "" {
			Challenge(w)
			return
		}

		if _, ok := anonymousPatterns[pattern]; ok {
			mux.ServeHTTP(w, r)
			return
		}

		token, ok := bearerToken(r)
		if !ok {
			Challenge(w)
			return
		}
		subject, err := v.ValidateToken(r.Context(), token)
		if err != nil {
			Challenge(w)
			return
		}

		mux.ServeHTTP(w, r.WithContext(WithSubject(r.Context(), subject)))
	})
}

// WithSubject returns a copy of ctx carrying the authenticated user's subject.
// RequireAuth calls it after a successful validation; tests use it to inject a
// subject when exercising a handler in isolation.
func WithSubject(ctx context.Context, subject string) context.Context {
	return context.WithValue(ctx, subjectKey{}, subject)
}

// Challenge writes the bodyless 401 that .NET's JwtBearer handler emits on an
// unauthenticated request: status 401 plus WWW-Authenticate: Bearer. The
// PascalCase error envelope is added downstream by middleware.ErrorBody, the
// same way ASP.NET's ErrorResponseMiddleware backfills the JwtBearer challenge.
func Challenge(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	w.WriteHeader(http.StatusUnauthorized)
}

// Subject returns the authenticated user's `sub` claim, or the empty string if
// the request was not authenticated (e.g. an anonymous route).
func Subject(ctx context.Context) string {
	if sub, ok := ctx.Value(subjectKey{}).(string); ok {
		return sub
	}
	return ""
}

// bearerToken extracts the token from an "Authorization: Bearer <token>"
// header. The scheme match is case-insensitive (RFC 7235), matching the .NET
// JwtBearer handler; an empty or malformed header yields ok=false.
func bearerToken(r *http.Request) (string, bool) {
	const prefix = "bearer "
	header := r.Header.Get("Authorization")
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	if token == "" {
		return "", false
	}
	return token, true
}
