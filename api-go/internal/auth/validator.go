package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/auth0/go-jwt-middleware/v3/jwks"
	"github.com/auth0/go-jwt-middleware/v3/validator"
)

// ErrUnconfigured is returned by a deny-all validator: the Auth0 domain or
// audience was absent, so no token can be validated and every request to a
// protected route is denied. This is fail-closed by design — the Go dev app
// ships without Auth0 env vars until infra wires them (GH#418, it3+), and a
// missing config must never let unauthenticated requests through.
var ErrUnconfigured = errors.New("auth0 validator not configured")

// jwksCacheTTL caps how long a fetched JWKS is trusted before a background
// refresh. 15 minutes matches the library default and the .NET handler's
// effective JWKS cache window.
const jwksCacheTTL = 15 * time.Minute

// allowedClockSkew tolerates minor clock drift between the API host and Auth0
// when validating exp/nbf, mirroring the small skew .NET's handler permits.
const allowedClockSkew = 30 * time.Second

// underlyingValidator is the slice of the library *validator.Validator the
// adapter calls. Keeping it as an interface lets the deny-all path hold a nil
// implementation without a type switch at the call site.
type underlyingValidator interface {
	ValidateToken(ctx context.Context, token string) (any, error)
}

// Auth0Validator adapts the auth0 go-jwt-middleware validator to the
// consumer-side tokenValidator interface, returning the JWT subject on success.
// It validates iss/aud/exp/nbf and verifies signatures against the issuer's
// JWKS (cached, background-refreshed). When constructed without Auth0 config it
// becomes a deny-all validator.
type Auth0Validator struct {
	inner  underlyingValidator
	logger *slog.Logger
}

// NewAuth0Validator builds a validator for the given Auth0 domain and API
// audience. When either is empty it returns a deny-all validator (inner == nil)
// rather than an error, so the API still boots and denies every protected
// request. The JWKS provider fetches lazily on first validation, preserving
// cold-start latency.
func NewAuth0Validator(domain, audience string, logger *slog.Logger) (*Auth0Validator, error) {
	if domain == "" || audience == "" {
		logger.Warn("auth0 validator unconfigured; all protected routes will deny",
			"hasDomain", domain != "", "hasAudience", audience != "")
		return &Auth0Validator{logger: logger}, nil
	}

	issuerURL, err := url.Parse("https://" + domain + "/")
	if err != nil {
		return nil, fmt.Errorf("parse issuer url for domain %q: %w", domain, err)
	}

	provider, err := jwks.NewCachingProvider(
		jwks.WithIssuerURL(issuerURL),
		jwks.WithCacheTTL(jwksCacheTTL),
	)
	if err != nil {
		return nil, fmt.Errorf("build jwks provider: %w", err)
	}

	inner, err := validator.New(
		validator.WithKeyFunc(provider.KeyFunc),
		validator.WithAlgorithm(validator.RS256),
		validator.WithIssuer(issuerURL.String()),
		validator.WithAudience(audience),
		validator.WithAllowedClockSkew(allowedClockSkew),
		validator.WithCustomClaims(func() validator.CustomClaims { return &profileClaims{} }),
	)
	if err != nil {
		return nil, fmt.Errorf("build jwt validator: %w", err)
	}

	return &Auth0Validator{inner: inner, logger: logger}, nil
}

// ValidateToken validates the raw token and returns its claims. A deny-all
// validator (no Auth0 config) returns ErrUnconfigured for any token.
func (a *Auth0Validator) ValidateToken(ctx context.Context, token string) (Claims, error) {
	if a.inner == nil {
		return Claims{}, ErrUnconfigured
	}
	validated, err := a.inner.ValidateToken(ctx, token)
	if err != nil {
		return Claims{}, fmt.Errorf("validate token: %w", err)
	}
	return claimsFromValidated(validated)
}

// profileClaims captures the non-registered claims the create-profile path
// reads: email, email_verified, and the (non-authoritative) subscription_tier.
// Validate is a no-op — these are informational, not security-critical.
type profileClaims struct {
	Email            string `json:"email"`
	EmailVerified    bool   `json:"email_verified"`
	SubscriptionTier string `json:"subscription_tier"`
}

func (*profileClaims) Validate(context.Context) error { return nil }

// claimsFromValidated maps the library's ValidatedClaims to our Claims. An
// unexpected type or an empty subject is an error, since every Auth0 access
// token carries a non-empty sub.
func claimsFromValidated(v any) (Claims, error) {
	validated, ok := v.(*validator.ValidatedClaims)
	if !ok {
		return Claims{}, fmt.Errorf("unexpected claims type %T", v)
	}
	if validated.RegisteredClaims.Subject == "" {
		return Claims{}, errors.New("token has no subject claim")
	}
	c := Claims{Subject: validated.RegisteredClaims.Subject}
	if pc, ok := validated.CustomClaims.(*profileClaims); ok && pc != nil {
		c.Email = pc.Email
		c.EmailVerified = pc.EmailVerified
		c.SubscriptionTier = pc.SubscriptionTier
	}
	return c, nil
}
