# 0011. Web Frontend Stack: Vite + React 19 + TypeScript

Date: 2026-03-19

## Status

Accepted

## Context

Town Crier needed a public-facing web presence for the landing page and marketing site. The existing monorepo contained only a .NET API backend and native iOS app. We needed to choose a frontend framework, build tool, hosting strategy, and styling approach for a new `/web` directory.

Key requirements:
- Fast static site performance (landing page is content-heavy, no SSR needed)
- Type safety consistent with the project's strict typing philosophy
- Low hosting cost at early-stage scale
- CI/CD with PR preview environments
- Design system consistency with the existing iOS app (shared design tokens)

## Decision

We adopt the following web frontend stack:

- **Framework:** React 19 with TypeScript 5.9 in strict mode (including `noUncheckedIndexedAccess`)
- **Build tool:** Vite 8 with the `@vitejs/plugin-react` plugin
- **Styling:** CSS Modules with CSS custom properties mapping to design tokens (colors, spacing, typography, radii, shadows), dark-first theming with light and OLED dark variants
- **Testing:** Vitest + Testing Library (jsdom environment)
- **Hosting:** Azure Static Web Apps (Free tier) with SPA fallback routing and security headers
- **CI/CD:** GitHub Actions — build and type-check on PR, deploy to staging preview on PR, deploy to production on merge to main

No SSR framework (Next.js, Remix) is used. The landing page is a pure client-side SPA served as static files, which keeps the build pipeline simple and the hosting free.

## Consequences

- **Simpler:** Pure SPA with Vite gives fast builds, minimal configuration, and zero server-side runtime costs. Azure Static Web Apps Free tier provides hosting, SSL, and PR preview environments at no cost.
- **Simpler:** CSS Modules with design tokens ensure visual consistency with the iOS app without introducing a component library dependency (e.g., Chakra, MUI).
- **Harder:** No SSR means the landing page relies on client-side rendering. If SEO or first-contentful-paint becomes critical, we would need to revisit this (e.g., adopt Vite SSG plugin or migrate to a framework with SSR support).
- **Harder:** Adding a second language ecosystem (Node.js/TypeScript) to the monorepo increases CI complexity and onboarding surface area.

## Amendments

### 2026-03-27
The web frontend has evolved from a landing page and marketing site into a **full authenticated application** with feature parity approaching the iOS app. The core technology choices (React 19, TypeScript 5.9, Vite 8, CSS Modules, Vitest) remain unchanged. The following additions reflect the expanded scope:

- Added: **React Router DOM** v7.13.2 for client-side routing. 13 routes spanning public pages (landing, legal, callback), onboarding, and authenticated features (dashboard, applications, watch zones, notifications, search, saved applications, settings, map). *Amended 2026-04-21: the original count of 17 included Group routes that were later removed along with the Groups feature — see [ADR 0008](0008-cosmos-db-data-model.md) 2026-04-03 amendment.*
- Added: **@auth0/auth0-react** v2.16.0 for authentication. Integrates with the same Auth0 tenant as the iOS app (see [ADR 0007](0007-auth0-authentication.md)). Route protection via `AuthGuard` and `OnboardingGate` components.
- Added: **TanStack React Query** v5.95.2 for server state management and data fetching. Currently used selectively (saved applications) alongside manual `useState`/`useEffect` patterns in other features.
- Added: **Leaflet** v1.9.4 with **react-leaflet** v5.0.0 for interactive maps. The `/map` route renders a full MapPage with OpenStreetMap tiles, watch zone overlays, and application markers with popups. Map components are also integrated into watch zone and application detail features.
- Added: **Port/Adapter architecture** in the web layer. Domain ports (interfaces) defined in `src/domain/ports/`, with API adapters composing typed API modules. Components receive repositories as props for testability ("Connected" components create adapters; presentational components receive them).
- Added: **Branded types** (`ApplicationUid`, `WatchZoneId`, `GroupId`, etc.) and **union types** (instead of enums, respecting `erasableSyntaxOnly`) for type-safe domain modelling in TypeScript.
- Updated: The web frontend is no longer just a public-facing landing page. It serves as the primary web client for authenticated users, with 14 implemented feature modules.

### 2026-04-21
- Corrected: route count is **13** (not 17) — the Groups feature and its routes were removed ([ADR 0008](0008-cosmos-db-data-model.md) 2026-04-03 amendment). A test in `domain/types` explicitly asserts no Group symbols are exported.
- Added: **Offer codes** feature (`features/offerCode/`). Redemption UI and `useRedeemOfferCode` hook integrate with the API offer-code endpoints (see ADR 0022).
- Clarified: **`@microsoft/applicationinsights-web` is not installed** in `web/package.json` and no telemetry is initialised in the browser bundle. The React `ErrorBoundary` renders a fallback UI but does not call `trackException`. [ADR 0018](0018-opentelemetry-observability-with-azure-monitor.md) describes frontend instrumentation that has not yet shipped; the 2026-04-21 amendment on 0018 records this.
- Clarified: React Query is still used selectively (saved applications, offer-code redemption). Most features continue to use manual `useState` / `useEffect` with a shared `useFetchData` hook.
- Added: **PWA manifest** (`site.webmanifest`) is served, but no service worker / offline cache is registered. Dark-mode theming is implemented via `useTheme` (localStorage persistence, `prefers-color-scheme` fallback, `data-theme` attribute).
