// Package auth provides Auth0 JWT bearer authentication and fallback-deny
// authorization (GH#418). Every route requires a valid Auth0 access token unless
// it was explicitly registered as anonymous; unmatched routes fall through to a
// 401 challenge (a "no endpoint -> fallback policy" flow).
package auth

import (
	"context"
	"net/http"
	"strings"
)

// Claims carries the JWT claims the handlers need after a token is validated:
// the subject plus the email, email-verified flag, and subscription_tier the
// create-profile path reads (mirroring .NET's ClaimsPrincipal lookups in
// UserProfileEndpoints). Email-derived claims are empty when the token omits
// them.
type Claims struct {
	Subject          string
	Email            string
	EmailVerified    bool
	SubscriptionTier string
}

// TokenValidator validates a raw bearer token and returns its claims. The
// concrete *Auth0Validator satisfies it; unit tests substitute a hand-written
// fake so no JWKS network call happens. It is exported because the binary's
// wiring (cmd/api) names it when assembling the router.
type TokenValidator interface {
	ValidateToken(ctx context.Context, token string) (Claims, error)
}

// routeMatcher is the subset of *http.ServeMux the middleware uses: it both
// reports which registered pattern (if any) a request matches — resolving the
// endpoint before the authorization decision — and dispatches the request once
// the auth decision is made.
type routeMatcher interface {
	http.Handler
	Handler(r *http.Request) (h http.Handler, pattern string)
}

type claimsKey struct{}

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
		claims, err := v.ValidateToken(r.Context(), token)
		if err != nil {
			Challenge(w)
			return
		}

		mux.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), claims)))
	})
}

// WithClaims returns a copy of ctx carrying the authenticated user's claims.
// RequireAuth calls it after a successful validation; tests use it to inject
// claims when exercising a handler in isolation.
func WithClaims(ctx context.Context, claims Claims) context.Context {
	return context.WithValue(ctx, claimsKey{}, claims)
}

// WithSubject is a convenience that threads only the subject, used by handler
// tests that do not exercise the email/tier claims.
func WithSubject(ctx context.Context, subject string) context.Context {
	return WithClaims(ctx, Claims{Subject: subject})
}

// ClaimsFrom returns the authenticated user's claims, or the zero Claims if the
// request was not authenticated (e.g. an anonymous route).
func ClaimsFrom(ctx context.Context) Claims {
	if c, ok := ctx.Value(claimsKey{}).(Claims); ok {
		return c
	}
	return Claims{}
}

// Challenge writes the bodyless 401 emitted on an unauthenticated request:
// status 401 plus WWW-Authenticate: Bearer. The PascalCase error envelope is
// added downstream by middleware.ErrorBody.
func Challenge(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	w.WriteHeader(http.StatusUnauthorized)
}

// Subject returns the authenticated user's `sub` claim, or the empty string if
// the request was not authenticated (e.g. an anonymous route).
func Subject(ctx context.Context) string {
	return ClaimsFrom(ctx).Subject
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
