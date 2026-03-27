# Town Crier Web Application — Full-Featured SPA

Date: 2026-03-26

## Overview

Build a full-featured web client with complete feature parity to the iOS app. The web app becomes the first working application for validating API functionality end-to-end, with the iOS app launching after. The existing landing page in `/web` evolves into a single SPA containing both the marketing site and the authenticated application.

## Architecture & Tech Stack

Static SPA built with the existing Vite + React + TypeScript setup in `/web`. No SSR. Hosted on Azure Static Web Apps (or Blob Storage + CDN) for near-zero cost.

| Concern | Choice |
|---------|--------|
| Framework | React 19 + TypeScript (already in place) |
| Routing | React Router v7 |
| Server state | TanStack Query |
| Auth | Auth0 SPA SDK (`@auth0/auth0-react`) — same tenant as iOS |
| Maps | Leaflet + OpenStreetMap tiles |
| Styling | CSS Modules + design tokens from design-language skill |
| Testing | Vitest + Testing Library |
| Hosting | Azure Static Web Apps |
| Payments | None — free tier only on web, Pro requires iOS |

No new API endpoints needed. The only backend change is adding CORS configuration.

## Navigation & Layout

Sidebar navigation with a dashboard home page. The sidebar lists all major sections and is always visible on desktop. On viewports under 768px it collapses to a hamburger overlay.

After login, the app shell is:

```
<AppShell>
  <Sidebar />       ← persistent left nav
  <Outlet />        ← page content via React Router
</AppShell>
```

## Route Structure

```
/                        → Landing page (marketing — existing)
/login                   → Auth0 redirect
/callback                → Auth0 callback handler
/onboarding              → Postcode + radius setup (post-signup)

/dashboard               → Home — watch zone summaries, recent activity
/applications            → Browse by authority
/applications/:uid       → Application detail
/search                  → Search applications
/map                     → Leaflet map view
/watch-zones             → List watch zones
/watch-zones/new         → Create watch zone
/watch-zones/:id         → Edit watch zone + preferences
/saved                   → Saved applications
/notifications           → Notification history
/groups                  → List groups
/groups/new              → Create group
/groups/:id              → Group detail + member management
/invitations/:id/accept  → Accept group invitation
/settings                → Profile, data export, delete account
/legal/:type             → Privacy policy, terms of service
```

Auth guard protects all routes except `/`, `/login`, `/callback`, and `/legal/:type`. Onboarding gate redirects new users (no profile) to `/onboarding` before allowing app access.

## Feature Breakdown

### Dashboard (`/dashboard`)
- Fetches watch zones (`GET /v1/me/watch-zones`) and displays summary cards
- Shows recent applications by fetching `GET /v1/applications?authorityId=` for each watch zone's authority
- Quick links to saved applications and notification history

### Applications (`/applications`)
- Authority selector with typeahead (`GET /v1/authorities?search=`)
- Application list for selected authority (`GET /v1/applications?authorityId=`)
- Click through to detail view

### Application Detail (`/applications/:uid`)
- Full detail view (`GET /v1/applications/:uid`)
- Save/unsave toggle (`PUT/DELETE /v1/me/saved-applications/:uid`)
- Designation context if coordinates available (`GET /v1/designations`)

### Search (`/search`)
- Search input + authority selector
- Paginated results (`GET /v1/search?q=&authorityId=&page=`)
- Pro tier gating — free tier users see a prompt to upgrade via iOS

### Map (`/map`)
- Leaflet + OpenStreetMap tiles
- Plots applications from the user's watch zones
- Click markers for summary, click through to detail

### Watch Zones (`/watch-zones`)
- List view with zone cards (`GET /v1/me/watch-zones`)
- Create: postcode geocoding → radius picker → authority resolution → `POST /v1/me/watch-zones`
- Edit preferences per zone (`GET/PUT /v1/me/watch-zones/:id/preferences`)
- Delete (`DELETE /v1/me/watch-zones/:id`)

### Saved Applications (`/saved`)
- List (`GET /v1/me/saved-applications`)
- Remove (`DELETE /v1/me/saved-applications/:uid`)
- Click through to detail

### Notifications (`/notifications`)
- Paginated list (`GET /v1/notifications?page=&pageSize=`)
- Click through to relevant application

### Groups (`/groups`)
- List user's groups (`GET /v1/groups`)
- Create group (`POST /v1/groups`) — postcode → radius → authority flow
- Group detail (`GET /v1/groups/:id`) — member list, invite form
- Invite member (`POST /v1/groups/:id/invitations`)
- Remove member (`DELETE /v1/groups/:id/members/:userId`)
- Delete group (`DELETE /v1/groups/:id`)
- Accept invitation (`POST /v1/invitations/:id/accept`) — deep link landing page

### Settings (`/settings`)
- View/edit profile (`GET/PATCH /v1/me`)
- Export data (`GET /v1/me/data`) — downloads as JSON
- Delete account (`DELETE /v1/me`) — confirmation dialog, then logout
- Theme toggle (light/dark/system)

### Legal (`/legal/:type`)
- Static content pages for privacy policy and terms of service

### Onboarding (`/onboarding`)
- Step 1: Enter postcode → geocode
- Step 2: Pick radius
- Step 3: Confirm → creates user profile + first watch zone
- Redirects to `/dashboard`

## Component Architecture

### Layout Hierarchy

```
<App>
  <AuthProvider>              ← Auth0 provider
    <QueryClientProvider>     ← TanStack Query
      <Router>
        <LandingPage />       ← "/" unauthenticated
        <AuthGuard>           ← all other routes
          <OnboardingGate>    ← redirects to /onboarding if no profile
            <AppShell>        ← sidebar + main content area
              <Sidebar />
              <Outlet />      ← page content via React Router
            </AppShell>
          </OnboardingGate>
        </AuthGuard>
      </Router>
    </QueryClientProvider>
  </AuthProvider>
</App>
```

### Shared Components
- `ApplicationCard` — used in lists, search results, saved apps, dashboard
- `AuthoritySelector` — typeahead, used in browse + search + zone/group creation
- `PostcodeInput` — geocoding input, used in onboarding + watch zone + group creation
- `RadiusPicker` — slider/select, used in onboarding + watch zone + group creation
- `Pagination` — used in applications, search, notifications
- `ConfirmDialog` — used for delete actions (zone, group, account)
- `EmptyState` — consistent empty state messaging
- `ProGate` — shown when a free-tier user hits a Pro feature, directs them to the iOS app

### Directory Structure

```
src/
  components/           ← landing page components (existing)
  features/             ← authenticated app features
    Dashboard/
    Applications/
    ApplicationDetail/
    Search/
    Map/
    WatchZones/
    SavedApplications/
    Notifications/
    Groups/
    Settings/
    Onboarding/
    Legal/
  shared/               ← reusable components
  hooks/                ← TanStack Query hooks
  api/                  ← API client (fetch wrapper with Auth0 token injection)
  domain/               ← TypeScript types mirroring API contracts
  auth/                 ← Auth0 config, AuthGuard, OnboardingGate
  styles/               ← design tokens, global styles (existing)
```

### API Client Pattern
- Thin `fetch` wrapper injecting the Auth0 bearer token via `getAccessTokenSilently()`
- Each domain area gets a module in `api/` (e.g., `api/applications.ts`, `api/watchZones.ts`)
- TanStack Query hooks in `hooks/` wrap the API calls with caching/refetching

## Testing Strategy

Hand-written test doubles throughout — no `jest.mock` or reflection-based mocking.

### Unit Tests
- **Hooks** — each TanStack Query hook tested with a mock API client. Verify query keys, request parameters, error handling.
- **Components** — render with Testing Library, assert on content and interactions. Mock hooks, not fetch.
- **API client** — test request construction, token injection, error mapping. Mock `fetch`.
- **Domain types** — test any validation or transformation logic.

### Test Doubles
- Hand-written fakes and spies
- Fixture builders for API response shapes (e.g., `buildApplication()`, `buildWatchZone()`)
- Fake API client for hook tests

### Integration Tests
- Auth flow — AuthGuard redirects, OnboardingGate behaviour, callback handling
- Key user journeys — onboarding, create watch zone, search and save application

### Not Tested
- Auth0 SDK internals
- Leaflet rendering (mock the map component in page tests)
- Landing page components (already tested)

## Key Technical Decisions

### Auth0 Configuration
- New "Single Page Application" client in the existing Auth0 tenant
- Allowed callback URL: `https://towncrierapp.uk/callback` (+ `http://localhost:5173` for dev)
- Allowed logout URL: `https://towncrierapp.uk`
- Same API audience as the iOS app — tokens are interchangeable, same user identity

### CORS
- API needs CORS headers allowing `https://towncrierapp.uk` and `http://localhost:5173`
- This is the only API change needed

### Theme Support
- Light, dark, and OLED dark themes from the design-language skill
- System preference detection + manual toggle (already exists in landing page)
- Persisted to `localStorage`

### Responsive Breakpoints
- Desktop: sidebar visible, multi-column layouts
- Tablet (<1024px): sidebar collapsible
- Mobile (<768px): sidebar becomes hamburger overlay, single-column layouts

### Deployment
- Vite build → static assets
- Azure Static Web Apps with fallback route (`/*` → `/index.html` for SPA routing)
- CI/CD via GitHub Actions (consistent with existing pipelines)

### Notifications
- No browser push notifications
- Notification history page shows what has been sent (read-only)

### Subscriptions
- Free tier only on web
- Pro features show a `ProGate` component directing users to the iOS app to upgrade
