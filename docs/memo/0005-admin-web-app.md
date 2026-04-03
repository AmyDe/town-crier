# 0005. Separate Admin Web App

Date: 2026-04-03

## Status

Open

## Question

Should we build a separate static web app for admin functionality (dashboards and user management), and how should admin access be modelled?

## Analysis

### Current State

- One admin endpoint exists: `PUT /admin/subscriptions` protected by a hardcoded API key (`X-Admin-Key` header)
- No RBAC, no admin role in the user profile or identity provider
- The API key is also used by CI/CD for `AUTO_GRANT_PRO_DOMAINS` grants
- The main web app at `towncrierapp.uk` / `dev.towncrierapp.uk` is a React 19 + Vite SPA deployed as an Azure Static Web App

### Proposed Admin Functionality

**Operational dashboards (read-only):**
- User counts, subscription stats, system health
- Planning application ingestion metrics

**User management (read-write):**
- Look up users by email or ID
- Grant/revoke subscriptions
- No impersonation

### Admin Identity Model

Auth0 RBAC was chosen over alternatives (domain-based allow list, UserProfile flag, or a combination). Rationale:

- Auth0 roles are purpose-built for identity-level access control
- The admin role changes rarely (manual assignment) — unlike subscription tier which has a rich lifecycle driven by the App Store
- The role claim travels with the JWT, so both the admin SPA and the API can use it without extra DB lookups
- Keeps admin logic out of the domain model, which should remain focused on subscription/notification concerns

**Auth0 free tier caveat:** RBAC features (roles, permissions, enforcement, Actions) all work on the free plan today, but Auth0's pricing page officially lists RBAC as a paid feature. The risk is that Auth0 could start enforcing the paywall, requiring the Essentials plan ($35/mo). For 1-2 admin users this is an acceptable risk.

**Why not use Auth0 roles for subscription tier?** Subscription tier (Free/Pro) was explicitly excluded from RBAC because it has a time-based lifecycle (activate, renew, grace period, expire) driven by App Store server-to-server notifications. Putting tier in Auth0 would create token staleness issues (user pays but token still says Free until refresh) and require background jobs to call Auth0's Management API on expiry. Tier remains a domain concern in the UserProfile model.

### Implementation Approach

A post-login Auth0 Action stamps the user's roles as a custom claim (e.g. `https://towncrierapp.uk/roles`) on the access token. The API validates this claim on admin endpoints. The admin SPA reads the claim client-side for UI gating, but server-side enforcement is the real security boundary.

## Options Considered

### 1. Separate static web app (recommended)

A new React 19 + Vite SPA deployed as its own Azure Static Web App at `admin.towncrierapp.uk`. Uses the same Auth0 SPA application registration as the main app (just add the admin subdomain to allowed callback/logout URLs). Admin vs user access is determined by the Auth0 role claim.

**Pros:**
- Clean separation of concerns — admin UI doesn't ship to end users
- Independent deploy cycle — admin changes don't require a user-facing release
- Follows existing DNS/infra pattern (new SWA + custom domain)
- Same tech stack (React 19, Vite, Auth0 React SDK, TanStack Query) — no new learning curve

**Cons:**
- Second SWA to provision and maintain
- Some shared code (auth adapter, API client) will need to be duplicated or extracted

### 2. Admin routes within the main web app

Add `/admin/*` routes to the existing SWA, gated by the role claim.

**Pros:**
- No new infrastructure
- Shared code is trivially shared

**Cons:**
- Admin code ships in the user-facing bundle (even if lazy-loaded)
- Coupled deploy cycle — admin changes require a user-facing release
- Route-level gating is easier to misconfigure than app-level separation

### 3. Separate API for admin

A second .NET container serving only admin endpoints.

**Pros:**
- Complete isolation of admin logic

**Cons:**
- Duplicates infrastructure (container, identity, Cosmos DB access)
- Same domain model, same database — separation is artificial
- Significantly more operational overhead for marginal benefit

## Recommendation

**Option 1 — separate static web app** with:

- `admin.towncrierapp.uk` (prod) and `admin-dev.towncrierapp.uk` (dev)
- Auth0 `admin` role via RBAC, stamped as a custom claim by a post-login Action
- Shared Auth0 SPA application registration (add admin callback URLs)
- Same tech stack as the main web app (React 19 + Vite + TanStack Query)
- Admin API endpoints remain in the existing API under the `/admin` route group
- Existing API key auth on `/admin` routes replaced by JWT + role claim validation (immediate cut-over, no transition period)
- CI/CD pipeline extended with an admin SWA build/deploy step
- The `AUTO_GRANT_PRO_DOMAINS` grant mechanism (currently using the API key) will need reworking — either moved into the admin app or converted to a service-to-service flow
