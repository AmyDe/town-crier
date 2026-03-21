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
