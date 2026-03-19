# Landing Page Design Spec

Date: 2026-03-19

## Overview

A static landing page at `towncrierapp.uk` to drive iOS App Store downloads. Built as a React app scaffolded for future growth into a full web application mirroring the iOS app's features. Purely static content for now — no API calls.

## Goals

- Drive iOS app downloads via App Store funnel
- Communicate Town Crier's value proposition with a civic, community-minded tone
- Show full pricing transparency (all three tiers)
- Scaffold a React app that grows into a full web client calling the existing .NET API

## Tech Stack

| Component | Choice | Rationale |
|-----------|--------|-----------|
| Framework | React + TypeScript | Future web app will need React; TypeScript matches project quality bar |
| Build tool | Vite | Fast builds, static output to `/dist`, tree-shaking via Rollup, minimal bundle |
| Styling | CSS Modules + CSS custom properties | Scoped styles, no runtime cost, iOS design tokens map to CSS variables |
| Hosting | Azure Static Web Apps (free tier) | Free managed SSL, global CDN, custom domains, GitHub Actions CI/CD, PR staging environments |
| Infrastructure | Pulumi (C#/.NET) | New `AzureStaticWebApp` resource in `/infra` |

## Project Structure

```
/web
  ├── index.html                    # Vite entry point
  ├── vite.config.ts
  ├── tsconfig.json
  ├── package.json
  ├── staticwebapp.config.json      # Azure SWA routing & headers
  ├── src/
  │   ├── main.tsx                  # React entry
  │   ├── App.tsx                   # Root component (landing page)
  │   ├── components/
  │   │   ├── Navbar/
  │   │   │   ├── Navbar.tsx
  │   │   │   └── Navbar.module.css
  │   │   ├── Hero/
  │   │   │   ├── Hero.tsx
  │   │   │   └── Hero.module.css
  │   │   ├── StatsBar/
  │   │   │   ├── StatsBar.tsx
  │   │   │   └── StatsBar.module.css
  │   │   ├── HowItWorks/
  │   │   │   ├── HowItWorks.tsx
  │   │   │   └── HowItWorks.module.css
  │   │   ├── CommunityGroups/
  │   │   │   ├── CommunityGroups.tsx
  │   │   │   └── CommunityGroups.module.css
  │   │   ├── Pricing/
  │   │   │   ├── Pricing.tsx
  │   │   │   └── Pricing.module.css
  │   │   ├── Faq/
  │   │   │   ├── Faq.tsx
  │   │   │   └── Faq.module.css
  │   │   ├── Footer/
  │   │   │   ├── Footer.tsx
  │   │   │   └── Footer.module.css
  │   │   └── ThemeToggle/
  │   │       ├── ThemeToggle.tsx
  │   │       └── ThemeToggle.module.css
  │   ├── styles/
  │   │   ├── tokens.css            # CSS custom properties (light + dark themes)
  │   │   └── global.css            # Reset, base typography, body styles
  │   └── assets/                   # App icon, App Store badge SVG, section icons
  └── public/                       # Favicon, robots.txt
```

## Page Sections (top to bottom)

### Navbar
- Fixed/sticky at top
- Town Crier name/logo on the left
- Anchor links: Features, Pricing, FAQ
- Dark/light theme toggle (sun/moon icon)
- "Download" CTA button (amber) linking to App Store
- Collapses to hamburger menu on mobile

### Hero
- Full-width dark section
- Headline: "Stay informed about what's being built in your neighbourhood"
- Subheading: "Town Crier monitors planning applications from 417 UK local authorities and delivers them straight to your phone."
- Primary CTA: App Store download badge
- Subtle downward scroll indicator

### Stats Bar
- Three key figures in a horizontal row, bold amber on background:
  - **417** Local Authorities
  - **Free** To Get Started
  - **Real-time** Push Alerts

### How It Works
- Three-step visual flow with icons and connecting visual elements:
  1. **Enter your postcode** — Set your location in seconds
  2. **Create a watch zone** — Choose a radius around the areas you care about
  3. **Get notified** — Receive push notifications when new applications appear

### Community Groups
- Highlight section for the shared watch zone feature
- Explains creating groups, inviting neighbours, coordinating responses
- Differentiates Town Crier from being just a notification tool

### Pricing
- Three-column comparison table:

| | Free | Personal (£1.99/mo) | Pro (£5.99/mo) |
|---|---|---|---|
| Watch Zones | 1 | 1 | Unlimited |
| Radius | 1 km | 5 km | 10 km |
| Notifications | 5/month | Unlimited | Unlimited |
| Search | Browse only | Browse + filter | Full-text search |
| Historical data | Forward only | Instant backfill | Instant backfill |
| Free trial | — | 7 days | — |

- Personal tier visually highlighted as the recommended option (amber border)

### FAQ
- Accordion-style expandable questions (pure CSS + minimal React state):
  - Where does the data come from?
  - Which areas are covered?
  - Is it really free?
  - Can I use it with my neighbours?
  - How quickly will I be notified?

### Footer
- Final CTA: "Your neighbourhood is changing. Stay informed." with App Store badge
- Copyright, privacy policy link, terms link

## Design Tokens

All tokens defined as CSS custom properties in `tokens.css`. Both light and dark values provided; theme controlled via a `data-theme` attribute on `<html>`.

### Colours

**Dark theme (default):**

| Token | Value | Purpose |
|-------|-------|---------|
| `--tc-background` | `#1A1A1E` | Page background |
| `--tc-surface` | `#242428` | Card/component background |
| `--tc-surface-elevated` | `#2E2E33` | Modals, elevated elements |
| `--tc-amber` | `#E9A620` | Primary accent |
| `--tc-amber-muted` | `rgba(233, 166, 32, 0.15)` | Low-emphasis accent |
| `--tc-amber-hover` | `#F0B83A` | Hover/pressed state for amber elements |
| `--tc-text-primary` | `#F1EFE9` | Headings and body text |
| `--tc-text-secondary` | `#9B9590` | Captions, metadata |
| `--tc-text-tertiary` | `#5C5852` | Placeholder, disabled |
| `--tc-text-on-accent` | `#1C1917` | Text on amber backgrounds |
| `--tc-border` | `#3A3A3F` | Dividers, card outlines |
| `--tc-border-focused` | `#E9A620` | Focus rings |

**Light theme:**

| Token | Value | Purpose |
|-------|-------|---------|
| `--tc-background` | `#FAF8F5` | Page background |
| `--tc-surface` | `#FFFFFF` | Card/component background |
| `--tc-surface-elevated` | `#FFFFFF` | Elevated elements |
| `--tc-amber` | `#D4910A` | Primary accent |
| `--tc-amber-muted` | `rgba(212, 145, 10, 0.15)` | Low-emphasis accent |
| `--tc-amber-hover` | `#B87A08` | Hover/pressed state for amber elements |
| `--tc-text-primary` | `#1C1917` | Headings and body text |
| `--tc-text-secondary` | `#6B6560` | Captions, metadata |
| `--tc-text-tertiary` | `#A39E98` | Placeholder, disabled |
| `--tc-text-on-accent` | `#FFFFFF` | Text on amber backgrounds |
| `--tc-border` | `#E8E4DF` | Dividers, card outlines |
| `--tc-border-focused` | `#D4910A` | Focus rings |

### Typography

Font: **Inter** via Google Fonts or self-hosted (per design system — SF Pro on iOS, Inter on web)

| Token | Value | Usage |
|-------|-------|-------|
| `--tc-text-hero` | `3rem` | Hero headline |
| `--tc-text-h2` | `2rem` | Section headings |
| `--tc-text-h3` | `1.25rem` | Card/step titles |
| `--tc-text-body` | `1rem` | Body copy |
| `--tc-text-small` | `0.875rem` | Captions, FAQ answers |

### Spacing

| Token | Value |
|-------|-------|
| `--tc-space-xs` | `0.25rem` |
| `--tc-space-sm` | `0.5rem` |
| `--tc-space-md` | `1rem` |
| `--tc-space-lg` | `1.5rem` |
| `--tc-space-xl` | `2rem` |
| `--tc-space-xxl` | `3rem` |

### Border Radius

| Token | Value |
|-------|-------|
| `--tc-radius-sm` | `8px` |
| `--tc-radius-md` | `12px` |
| `--tc-radius-lg` | `16px` |
| `--tc-radius-full` | `9999px` |

### Layout

- Max content width: `1120px`, centred
- Sections: generous vertical padding (`--tc-space-xxl`)
- Responsive breakpoints:
  - Mobile: < 640px (single column)
  - Tablet: 640–1024px (2 columns where appropriate)
  - Desktop: > 1024px (full layout)

## Theme Toggle

- Sun/moon icon in the navbar
- First visit: respects `prefers-color-scheme` media query (user sees whatever their OS dictates)
- Manual toggle overrides and persists the choice in `localStorage`
- Subsequent visits: `localStorage` value takes precedence over `prefers-color-scheme`
- Toggles `data-theme="light"` / `data-theme="dark"` on `<html>`
- All components reference CSS custom properties, so theme switch is instant with no re-render

## Build & Deployment

### Development
- `cd web && npm run dev` — Vite dev server with hot reload
- `cd web && npm run build` — Production build to `/web/dist`

### Azure Static Web Apps
- Free tier: custom domain (`towncrierapp.uk`), global CDN, managed SSL certificates (Azure-native, no Let's Encrypt)
- SPA fallback: all routes serve `index.html` (configured in `staticwebapp.config.json`)
- PR builds get automatic staging environments

### GitHub Actions
- New workflow: `.github/workflows/web.yml`
- Path filter: `/web/**`
- Steps: install → build → deploy via `Azure/static-web-apps-deploy@v1`
- Triggered on push to main and on pull requests

### Pulumi
- New `AzureStaticWebApp` resource in `/infra`
- Custom domain binding for `towncrierapp.uk`

## Explicitly Not Included (YAGNI)

- No React Router — single page, add when web app grows
- No state management (Redux, Zustand) — static content
- No ESLint/Prettier — add when codebase warrants it
- No testing framework — static content has nothing to test; add Vitest when interactive features arrive
- No analytics or cookie banners — add when needed
- No SEO meta tags beyond basics (title, description, OG image)
- No OLED dark theme — this is an iOS-specific sub-mode for battery savings on OLED screens; not relevant to web
- No API integration — add when exposing features from the iOS app
