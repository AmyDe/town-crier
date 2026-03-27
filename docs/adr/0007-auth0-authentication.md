# 0007. Auth0 for Authentication

Date: 2026-03-16

## Status

Accepted

## Context

Town Crier needs user authentication for registration, login, and securing API endpoints. The authentication solution must support:

- Username/password registration and login
- Passkeys and TOTP-based multi-factor authentication
- Sign in with Apple (required for iOS apps offering third-party login)
- JWT-based API authentication compatible with .NET Native AOT
- A native iOS SDK compatible with SPM and SwiftUI

As a solo-developer project with a near-zero early budget (~£17–32/mo baseline), build-vs-buy economics strongly favour a managed service over self-hosted identity infrastructure (e.g., Keycloak, Duende IdentityServer). Self-hosted solutions carry ongoing maintenance burden, security patching responsibility, and infrastructure cost that are disproportionate at this scale.

### Alternatives Considered

| Option | Pros | Cons | Verdict |
|--------|------|------|---------|
| **Auth0** | 25K MAU free tier; passkeys, MFA, Sign in with Apple on free tier; actively maintained Swift and .NET SDKs; managed security patching | Vendor lock-in; free tier could change; token customisation limits on free plan | **Selected** |
| **Firebase Auth** | Generous free tier; good mobile SDKs | Google ecosystem lock-in; weaker .NET support; no passkey support on free tier at time of evaluation; less control over token claims | Rejected |
| **AWS Cognito** | Generous free tier (50K MAU); good SDK support | Complex configuration; poor developer experience; no native passkey support; pulls toward AWS when infra is Azure | Rejected |
| **Duende IdentityServer** | Full control; .NET native; no vendor dependency | Commercial licence required; self-hosted infrastructure cost; security patching responsibility; significant development effort | Rejected |
| **Keycloak** | Open source; feature-rich; no licence cost | Java-based; heavy resource footprint; self-hosted operational burden; no official Swift SDK | Rejected |
| **ASP.NET Core Identity (self-rolled)** | Zero vendor dependency; .NET native | Massive development effort for MFA, passkeys, Sign in with Apple; ongoing security responsibility; no iOS SDK | Rejected |

## Decision

We will use **Auth0** (managed, not self-hosted) as the authentication provider for Town Crier.

### Tenant Strategy

- **Development:** `town-crier-dev` tenant — used for local development and CI/CD integration tests.
- **Production:** `town-crier-prod` tenant — isolated from dev, with stricter security settings.

Separate tenants ensure that test accounts, configuration changes, and rate limit consumption in development cannot affect production users.

### Auth0 Applications

Two applications will be registered in each tenant:

1. **Town Crier API** (type: API)
   - Identifier/Audience: `https://api.towncrier.app` (or equivalent)
   - Used by the .NET backend to validate incoming JWTs
   - Token lifetime and scoping configured here

2. **Town Crier iOS** (type: Native)
   - Used by the iOS app via Auth0.swift SDK
   - Configured with Universal Links callback URLs
   - Refresh token rotation enabled

### Authentication Features (All Free Tier)

| Feature | Auth0 Configuration | Notes |
|---------|-------------------|-------|
| Username/password | Database Connection (default) | Standard email + password registration |
| Passkeys | Authentication → Passwordless | WebAuthn/FIDO2; supported on iOS 16+ |
| TOTP MFA | Security → Multi-factor Auth | Time-based one-time passwords via authenticator apps |
| Sign in with Apple | Authentication → Social → Apple | Requires Apple Developer account credentials; mandatory per App Store guidelines when offering third-party login |

### Integration Points

**iOS App (Auth0.swift SDK via SPM):**
- Universal Links for authentication callbacks (no custom URL schemes)
- Credential Manager integration for passkeys
- Secure token storage in iOS Keychain
- Refresh token rotation for session persistence

**.NET API (JWT validation):**
- `Microsoft.AspNetCore.Authentication.JwtBearer` middleware — no Auth0-specific SDK needed on the API side
- Validates tokens against Auth0's JWKS endpoint (`https://{domain}/.well-known/jwks.json`)
- Standard JWT claims (`sub`, `aud`, `iss`, `exp`) for authorisation
- Native AOT compatible — no reflection required

### Configuration Values

Each environment requires three values (stored in Azure Key Vault for production, user-secrets/environment variables for development):

| Value | Example | Used By |
|-------|---------|---------|
| Domain | `town-crier-dev.auth0.com` | iOS app, .NET API |
| Client ID | `abc123...` | iOS app only |
| API Audience | `https://api.towncrier.app` | .NET API (JWT `aud` claim) |

No client secret is needed — native mobile apps use PKCE (Proof Key for Code Exchange) instead of client secrets.

### Free Tier Limits

Auth0's free tier (as of March 2026) provides:

- 25,000 monthly active users
- Unlimited logins
- Passkeys, MFA, social connections
- 2 tenants (dev + prod)
- Community support only (no SLA)

At Town Crier's projected growth rate, the 25K MAU ceiling is unlikely to be reached for years. If it is, that represents a successful product generating subscription revenue to fund a paid Auth0 plan.

## Consequences

### What becomes easier

- **No authentication infrastructure to build or maintain** — Auth0 handles password hashing, brute-force protection, breached password detection, and security patching.
- **Passkeys, MFA, and Sign in with Apple from day one** — these are configuration toggles, not months of custom development.
- **Standards-based integration** — JWT validation on the API side uses standard ASP.NET Core middleware with no Auth0-specific dependencies.
- **Fast onboarding** — contributors can run the app locally against the dev tenant without setting up authentication infrastructure.

### What becomes harder

- **Vendor lock-in** — user credentials are stored in Auth0. Migration to another provider requires re-registering all users or implementing an export/import process. Mitigated by the fact that Auth0 supports standard OIDC, so the API's JWT validation is provider-agnostic.
- **Free tier dependency** — if Auth0 changes pricing or removes free tier features, we must migrate or pay. Mitigated by low switching cost on the API side (standard JWT) and moderate cost on the iOS side (swap Auth0.swift for another SDK).
- **Limited customisation on free plan** — no custom domains, limited Actions/Rules, basic email templates. Acceptable for MVP; revisit if branding requirements increase.
- **No SLA on free tier** — Auth0 outages would prevent login. Mitigated by refresh token rotation (existing sessions survive short outages) and by the fact that planning data viewing could be made available without authentication in a degraded mode.

## Amendments

### 2026-03-27
- Updated: Auth0 uses a **custom domain** (`towncrierapp.uk.auth0.com`) rather than separate default tenants (`town-crier-dev.auth0.com` / `town-crier-prod.auth0.com`). Dev and prod environments are differentiated by **API audience** (`https://api-dev.towncrierapp.uk` vs `https://api.towncrierapp.uk`), not by tenant.
- Added: **Town Crier Web** application (type: SPA) registered in Auth0, using `@auth0/auth0-react` v2.16.0. The web app uses the same Auth0 domain and differentiates environments via audience. Auth integration follows a port/adapter pattern (`Auth0AuthAdapter` implements `AuthPort`) with `AuthGuard` and `OnboardingGate` route protection components.
