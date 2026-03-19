# Landing Page Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a static landing page at `towncrierapp.uk` to drive iOS App Store downloads, scaffolded as a React app for future growth.

**Architecture:** Vite + React + TypeScript with CSS Modules. Design tokens from the iOS design system mapped to CSS custom properties. Dark/light theme via `data-theme` attribute on `<html>`. Deployed as an Azure Static Web App with GitHub Actions CI/CD.

**Tech Stack:** React 19, TypeScript (strict), Vite, CSS Modules, Azure Static Web Apps, Pulumi (C#), GitHub Actions

**Spec:** `docs/superpowers/specs/2026-03-19-landing-page-design.md`

**Coding Standards:** `@react-coding-standards` — named exports only, no inline styles, no `any`, semantic HTML, camelCase CSS Module classes, `var(--tc-*)` tokens for all visual values, no barrel files.

---

### Task 1: Scaffold Vite + React + TypeScript Project

**Files:**
- Create: `web/package.json`
- Create: `web/vite.config.ts`
- Create: `web/tsconfig.json`
- Create: `web/tsconfig.node.json`
- Create: `web/index.html`
- Create: `web/src/main.tsx`
- Create: `web/src/App.tsx`
- Create: `web/src/vite-env.d.ts`

- [ ] **Step 1: Scaffold the project with Vite**

```bash
cd /Users/christy/Dev/town-crier
npm create vite@latest web -- --template react-ts
```

- [ ] **Step 2: Install dependencies**

```bash
cd /Users/christy/Dev/town-crier/web && npm install
```

- [ ] **Step 3: Tighten TypeScript config**

Edit `web/tsconfig.json` (or `web/tsconfig.app.json` depending on Vite's template) to ensure these compiler options are set:

```json
{
  "compilerOptions": {
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "forceConsistentCasingInFileNames": true
  }
}
```

- [ ] **Step 4: Clean up Vite boilerplate**

Remove the default Vite template content:
- Delete `web/src/App.css`
- Delete `web/src/index.css`
- Delete `web/src/assets/react.svg`
- Delete `web/public/vite.svg`

Replace `web/src/App.tsx` with a minimal shell:

```tsx
export function App() {
  return (
    <main>
      <h1>Town Crier</h1>
    </main>
  );
}
```

Replace `web/src/main.tsx` with:

```tsx
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";

const root = document.getElementById("root");
if (!root) throw new Error("Root element not found");

createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>
);
```

Update `web/index.html`:
- Change `<title>` to `Town Crier — Planning Application Alerts`
- Add meta description: `Monitor UK planning applications and get real-time push notifications. 417 local authorities. Free to start.`
- Add Open Graph tags: `og:title`, `og:description`, `og:type` (website)
- Add `<link>` to Google Fonts for Inter (weights 400, 500, 600, 700)
- Remove any references to deleted files (vite.svg)

- [ ] **Step 5: Verify build**

```bash
cd /Users/christy/Dev/town-crier/web && npm run build && npx tsc --noEmit
```

Expected: Build succeeds with no errors.

- [ ] **Step 6: Verify dev server**

```bash
cd /Users/christy/Dev/town-crier/web && npm run dev
```

Expected: Dev server starts, page shows "Town Crier" heading at localhost.

- [ ] **Step 7: Commit**

```bash
git add web/
git commit -m "Scaffold Vite + React + TypeScript project for web landing page"
```

---

### Task 2: Design Tokens and Global Styles

**Files:**
- Create: `web/src/styles/tokens.css`
- Create: `web/src/styles/global.css`
- Modify: `web/src/main.tsx` (import styles)

- [ ] **Step 1: Create tokens.css**

Create `web/src/styles/tokens.css` with all design system tokens as CSS custom properties. Dark theme is the default (applied to `:root`). Light theme is applied via `[data-theme="light"]`. Also add `@media (prefers-color-scheme: light)` for first-visit detection before JS loads.

```css
/* Dark theme (default) */
:root {
  /* Brand */
  --tc-amber: #E9A620;
  --tc-amber-muted: rgba(233, 166, 32, 0.15);
  --tc-amber-hover: #F0B83A;

  /* Surfaces */
  --tc-background: #1A1A1E;
  --tc-surface: #242428;
  --tc-surface-elevated: #2E2E33;

  /* Text */
  --tc-text-primary: #F1EFE9;
  --tc-text-secondary: #9B9590;
  --tc-text-tertiary: #5C5852;
  --tc-text-on-accent: #1C1917;

  /* Utility */
  --tc-border: #3A3A3F;
  --tc-border-focused: #E9A620;

  /* Typography */
  --tc-font-family: "Inter", -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  --tc-text-hero: 3rem;
  --tc-text-h2: 2rem;
  --tc-text-h3: 1.25rem;
  --tc-text-body: 1rem;
  --tc-text-small: 0.875rem;

  /* Spacing */
  --tc-space-xs: 0.25rem;
  --tc-space-sm: 0.5rem;
  --tc-space-md: 1rem;
  --tc-space-lg: 1.5rem;
  --tc-space-xl: 2rem;
  --tc-space-xxl: 3rem;

  /* Border Radius */
  --tc-radius-sm: 8px;
  --tc-radius-md: 12px;
  --tc-radius-lg: 16px;
  --tc-radius-full: 9999px;

  /* Layout */
  --tc-max-width: 1120px;
}

/* Light theme */
[data-theme="light"] {
  --tc-amber: #D4910A;
  --tc-amber-muted: rgba(212, 145, 10, 0.15);
  --tc-amber-hover: #B87A08;

  --tc-background: #FAF8F5;
  --tc-surface: #FFFFFF;
  --tc-surface-elevated: #FFFFFF;

  --tc-text-primary: #1C1917;
  --tc-text-secondary: #6B6560;
  --tc-text-tertiary: #A39E98;
  --tc-text-on-accent: #FFFFFF;

  --tc-border: #E8E4DF;
  --tc-border-focused: #D4910A;
}

/* First-visit: respect OS preference before JS loads */
@media (prefers-color-scheme: light) {
  :root:not([data-theme]) {
    --tc-amber: #D4910A;
    --tc-amber-muted: rgba(212, 145, 10, 0.15);
    --tc-amber-hover: #B87A08;

    --tc-background: #FAF8F5;
    --tc-surface: #FFFFFF;
    --tc-surface-elevated: #FFFFFF;

    --tc-text-primary: #1C1917;
    --tc-text-secondary: #6B6560;
    --tc-text-tertiary: #A39E98;
    --tc-text-on-accent: #FFFFFF;

    --tc-border: #E8E4DF;
    --tc-border-focused: #D4910A;
  }
}
```

- [ ] **Step 2: Create global.css**

Create `web/src/styles/global.css` with CSS reset and base typography:

```css
*,
*::before,
*::after {
  box-sizing: border-box;
  margin: 0;
  padding: 0;
}

html {
  scroll-behavior: smooth;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

body {
  font-family: var(--tc-font-family);
  font-size: var(--tc-text-body);
  line-height: 1.6;
  color: var(--tc-text-primary);
  background-color: var(--tc-background);
  transition: background-color 0.2s ease, color 0.2s ease;
}

a {
  color: var(--tc-amber);
  text-decoration: none;
}

a:hover {
  color: var(--tc-amber-hover);
}

img {
  max-width: 100%;
  display: block;
}

button {
  font-family: inherit;
  cursor: pointer;
}
```

- [ ] **Step 3: Import styles in main.tsx**

Add to the top of `web/src/main.tsx`, before the App import:

```tsx
import "./styles/tokens.css";
import "./styles/global.css";
```

- [ ] **Step 4: Verify build**

```bash
cd /Users/christy/Dev/town-crier/web && npm run build && npx tsc --noEmit
```

Expected: Build succeeds. Dev server shows dark background with correct Inter font.

- [ ] **Step 5: Commit**

```bash
git add web/src/styles/
git commit -m "Add design tokens and global styles mapped from iOS design system"
```

---

### Task 3: Theme Toggle Hook

**Files:**
- Create: `web/src/hooks/useTheme.ts`

This hook manages theme state: reads `localStorage` on mount (falling back to `prefers-color-scheme`), sets `data-theme` on `<html>`, and exposes a toggle function.

- [ ] **Step 1: Create useTheme hook**

```typescript
// web/src/hooks/useTheme.ts
import { useState, useEffect, useCallback } from "react";

type Theme = "light" | "dark";

const STORAGE_KEY = "tc-theme";

function getInitialTheme(): Theme {
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored === "light" || stored === "dark") return stored;

  return window.matchMedia("(prefers-color-scheme: light)").matches
    ? "light"
    : "dark";
}

export function useTheme() {
  const [theme, setTheme] = useState<Theme>(getInitialTheme);

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", theme);
    localStorage.setItem(STORAGE_KEY, theme);
  }, [theme]);

  const toggleTheme = useCallback(() => {
    setTheme((prev) => (prev === "dark" ? "light" : "dark"));
  }, []);

  return { theme, toggleTheme } as const;
}
```

- [ ] **Step 2: Verify build**

```bash
cd /Users/christy/Dev/town-crier/web && npx tsc --noEmit
```

Expected: No type errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/hooks/useTheme.ts
git commit -m "Add useTheme hook for dark/light toggle with localStorage persistence"
```

---

### Task 4: Navbar Component

**Files:**
- Create: `web/src/components/Navbar/Navbar.tsx`
- Create: `web/src/components/Navbar/Navbar.module.css`
- Create: `web/src/components/ThemeToggle/ThemeToggle.tsx`
- Create: `web/src/components/ThemeToggle/ThemeToggle.module.css`

- [ ] **Step 1: Create ThemeToggle component**

```tsx
// web/src/components/ThemeToggle/ThemeToggle.tsx
import styles from "./ThemeToggle.module.css";

interface Props {
  theme: "light" | "dark";
  onToggle: () => void;
}

export function ThemeToggle({ theme, onToggle }: Props) {
  return (
    <button
      className={styles.toggle}
      onClick={onToggle}
      aria-label={`Switch to ${theme === "dark" ? "light" : "dark"} mode`}
      type="button"
    >
      {theme === "dark" ? "☀️" : "🌙"}
    </button>
  );
}
```

```css
/* web/src/components/ThemeToggle/ThemeToggle.module.css */
.toggle {
  background: none;
  border: 1px solid var(--tc-border);
  border-radius: var(--tc-radius-sm);
  padding: var(--tc-space-xs) var(--tc-space-sm);
  font-size: 1.25rem;
  line-height: 1;
  color: var(--tc-text-primary);
  transition: border-color 0.2s ease;
}

.toggle:hover {
  border-color: var(--tc-amber);
}

.toggle:focus-visible {
  outline: 2px solid var(--tc-border-focused);
  outline-offset: 2px;
}
```

- [ ] **Step 2: Create Navbar component**

The navbar is fixed/sticky, with anchor links, theme toggle, and a hamburger menu on mobile. Uses `<nav>` with semantic HTML.

```tsx
// web/src/components/Navbar/Navbar.tsx
import { useState } from "react";
import { ThemeToggle } from "../ThemeToggle/ThemeToggle";
import styles from "./Navbar.module.css";

interface Props {
  theme: "light" | "dark";
  onThemeToggle: () => void;
}

const APP_STORE_URL = "https://apps.apple.com/app/town-crier/id000000000"; // TODO: Replace with real App Store URL

export function Navbar({ theme, onThemeToggle }: Props) {
  const [menuOpen, setMenuOpen] = useState(false);

  return (
    <nav className={styles.navbar} aria-label="Main navigation">
      <div className={styles.inner}>
        <a href="#" className={styles.logo}>
          Town Crier
        </a>

        <button
          className={styles.hamburger}
          onClick={() => setMenuOpen((prev) => !prev)}
          aria-label={menuOpen ? "Close menu" : "Open menu"}
          aria-expanded={menuOpen}
          type="button"
        >
          <span className={styles.hamburgerLine} />
          <span className={styles.hamburgerLine} />
          <span className={styles.hamburgerLine} />
        </button>

        <div className={`${styles.links} ${menuOpen ? styles.open : ""}`}>
          <a href="#how-it-works" className={styles.link} onClick={() => setMenuOpen(false)}>
            Features
          </a>
          <a href="#pricing" className={styles.link} onClick={() => setMenuOpen(false)}>
            Pricing
          </a>
          <a href="#faq" className={styles.link} onClick={() => setMenuOpen(false)}>
            FAQ
          </a>
          <ThemeToggle theme={theme} onToggle={onThemeToggle} />
          <a href={APP_STORE_URL} className={styles.cta} target="_blank" rel="noopener noreferrer">
            Download
          </a>
        </div>
      </div>
    </nav>
  );
}
```

```css
/* web/src/components/Navbar/Navbar.module.css */
.navbar {
  position: sticky;
  top: 0;
  z-index: 100;
  background-color: var(--tc-background);
  border-bottom: 1px solid var(--tc-border);
  padding: var(--tc-space-sm) var(--tc-space-md);
}

.inner {
  max-width: var(--tc-max-width);
  margin: 0 auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.logo {
  font-size: var(--tc-text-h3);
  font-weight: 700;
  color: var(--tc-amber);
  text-decoration: none;
}

.logo:hover {
  color: var(--tc-amber-hover);
}

.links {
  display: flex;
  align-items: center;
  gap: var(--tc-space-lg);
}

.link {
  font-size: var(--tc-text-small);
  color: var(--tc-text-secondary);
  text-decoration: none;
  transition: color 0.2s ease;
}

.link:hover {
  color: var(--tc-text-primary);
}

.cta {
  font-size: var(--tc-text-small);
  font-weight: 600;
  color: var(--tc-text-on-accent);
  background-color: var(--tc-amber);
  padding: var(--tc-space-xs) var(--tc-space-md);
  border-radius: var(--tc-radius-sm);
  text-decoration: none;
  transition: background-color 0.2s ease;
}

.cta:hover {
  background-color: var(--tc-amber-hover);
  color: var(--tc-text-on-accent);
}

.hamburger {
  display: none;
  flex-direction: column;
  gap: 4px;
  background: none;
  border: none;
  padding: var(--tc-space-xs);
}

.hamburgerLine {
  display: block;
  width: 24px;
  height: 2px;
  background-color: var(--tc-text-primary);
  border-radius: 1px;
}

/* Mobile */
@media (max-width: 639px) {
  .hamburger {
    display: flex;
  }

  .links {
    display: none;
    flex-direction: column;
    position: absolute;
    top: 100%;
    left: 0;
    right: 0;
    background-color: var(--tc-surface);
    border-bottom: 1px solid var(--tc-border);
    padding: var(--tc-space-md);
    gap: var(--tc-space-md);
  }

  .open {
    display: flex;
  }
}
```

- [ ] **Step 3: Verify build**

```bash
cd /Users/christy/Dev/town-crier/web && npm run build && npx tsc --noEmit
```

Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add web/src/components/Navbar/ web/src/components/ThemeToggle/
git commit -m "Add Navbar with theme toggle, anchor links, and mobile hamburger menu"
```

---

### Task 5: Hero Component

**Files:**
- Create: `web/src/components/Hero/Hero.tsx`
- Create: `web/src/components/Hero/Hero.module.css`

- [ ] **Step 1: Create Hero component**

The hero section has a large headline, subheading, App Store badge CTA, and a scroll indicator.

```tsx
// web/src/components/Hero/Hero.tsx
import styles from "./Hero.module.css";

const APP_STORE_URL = "https://apps.apple.com/app/town-crier/id000000000"; // TODO: Replace with real App Store URL

export function Hero() {
  return (
    <section className={styles.hero}>
      <div className={styles.inner}>
        <h1 className={styles.headline}>
          Stay informed about what&rsquo;s being built in your neighbourhood
        </h1>
        <p className={styles.subheading}>
          Town Crier monitors planning applications from 417 UK local
          authorities and delivers them straight to your phone.
        </p>
        <a
          href={APP_STORE_URL}
          className={styles.cta}
          target="_blank"
          rel="noopener noreferrer"
          aria-label="Download Town Crier on the App Store"
        >
          Download on the App Store
        </a>
      </div>
      <div className={styles.scrollIndicator} aria-hidden="true">
        ↓
      </div>
    </section>
  );
}
```

```css
/* web/src/components/Hero/Hero.module.css */
.hero {
  min-height: 80vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: var(--tc-space-xxl) var(--tc-space-md);
  position: relative;
}

.inner {
  max-width: 720px;
}

.headline {
  font-size: var(--tc-text-hero);
  font-weight: 700;
  line-height: 1.15;
  color: var(--tc-text-primary);
  margin-bottom: var(--tc-space-lg);
}

.subheading {
  font-size: var(--tc-text-h3);
  color: var(--tc-text-secondary);
  line-height: 1.5;
  margin-bottom: var(--tc-space-xl);
}

.cta {
  display: inline-block;
  font-size: var(--tc-text-body);
  font-weight: 600;
  color: var(--tc-text-on-accent);
  background-color: var(--tc-amber);
  padding: var(--tc-space-sm) var(--tc-space-xl);
  border-radius: var(--tc-radius-md);
  text-decoration: none;
  transition: background-color 0.2s ease;
}

.cta:hover {
  background-color: var(--tc-amber-hover);
  color: var(--tc-text-on-accent);
}

.scrollIndicator {
  position: absolute;
  bottom: var(--tc-space-xl);
  font-size: 1.5rem;
  color: var(--tc-text-tertiary);
  animation: bounce 2s infinite;
}

@keyframes bounce {
  0%, 20%, 50%, 80%, 100% {
    transform: translateY(0);
  }
  40% {
    transform: translateY(-8px);
  }
  60% {
    transform: translateY(-4px);
  }
}

@media (max-width: 639px) {
  .headline {
    font-size: 2rem;
  }

  .subheading {
    font-size: var(--tc-text-body);
  }
}
```

- [ ] **Step 2: Verify build**

```bash
cd /Users/christy/Dev/town-crier/web && npm run build && npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add web/src/components/Hero/
git commit -m "Add Hero section with headline, subheading, and App Store CTA"
```

---

### Task 6: StatsBar Component

**Files:**
- Create: `web/src/components/StatsBar/StatsBar.tsx`
- Create: `web/src/components/StatsBar/StatsBar.module.css`

- [ ] **Step 1: Create StatsBar component**

Three stats in a row: 417 Local Authorities, Free To Get Started, Real-time Push Alerts.

```tsx
// web/src/components/StatsBar/StatsBar.tsx
import styles from "./StatsBar.module.css";

const STATS = [
  { value: "417", label: "Local Authorities" },
  { value: "Free", label: "To Get Started" },
  { value: "Real-time", label: "Push Alerts" },
] as const;

export function StatsBar() {
  return (
    <section className={styles.statsBar} aria-label="Key statistics">
      <div className={styles.inner}>
        {STATS.map((stat) => (
          <div key={stat.label} className={styles.stat}>
            <span className={styles.value}>{stat.value}</span>
            <span className={styles.label}>{stat.label}</span>
          </div>
        ))}
      </div>
    </section>
  );
}
```

```css
/* web/src/components/StatsBar/StatsBar.module.css */
.statsBar {
  padding: var(--tc-space-xl) var(--tc-space-md);
  border-top: 1px solid var(--tc-border);
  border-bottom: 1px solid var(--tc-border);
}

.inner {
  max-width: var(--tc-max-width);
  margin: 0 auto;
  display: flex;
  justify-content: center;
  gap: var(--tc-space-xxl);
}

.stat {
  text-align: center;
}

.value {
  display: block;
  font-size: var(--tc-text-h2);
  font-weight: 700;
  color: var(--tc-amber);
}

.label {
  font-size: var(--tc-text-small);
  color: var(--tc-text-secondary);
}

@media (max-width: 639px) {
  .inner {
    gap: var(--tc-space-lg);
  }

  .value {
    font-size: var(--tc-text-h3);
  }
}
```

- [ ] **Step 2: Verify build**

```bash
cd /Users/christy/Dev/town-crier/web && npm run build && npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add web/src/components/StatsBar/
git commit -m "Add StatsBar with key figures (authorities, free, real-time)"
```

---

### Task 7: HowItWorks Component

**Files:**
- Create: `web/src/components/HowItWorks/HowItWorks.tsx`
- Create: `web/src/components/HowItWorks/HowItWorks.module.css`

- [ ] **Step 1: Create HowItWorks component**

Three-step flow: Enter postcode, Create watch zone, Get notified.

```tsx
// web/src/components/HowItWorks/HowItWorks.tsx
import styles from "./HowItWorks.module.css";

const STEPS = [
  {
    number: "1",
    title: "Enter your postcode",
    description: "Set your location in seconds",
    icon: "📍",
  },
  {
    number: "2",
    title: "Create a watch zone",
    description: "Choose a radius around the areas you care about",
    icon: "🔔",
  },
  {
    number: "3",
    title: "Get notified",
    description: "Receive push notifications when new applications appear",
    icon: "📋",
  },
] as const;

export function HowItWorks() {
  return (
    <section id="how-it-works" className={styles.section}>
      <div className={styles.inner}>
        <h2 className={styles.heading}>How it works</h2>
        <ol className={styles.steps}>
          {STEPS.map((step) => (
            <li key={step.number} className={styles.step}>
              <span className={styles.icon} aria-hidden="true">
                {step.icon}
              </span>
              <h3 className={styles.title}>{step.title}</h3>
              <p className={styles.description}>{step.description}</p>
            </li>
          ))}
        </ol>
      </div>
    </section>
  );
}
```

```css
/* web/src/components/HowItWorks/HowItWorks.module.css */
.section {
  padding: var(--tc-space-xxl) var(--tc-space-md);
}

.inner {
  max-width: var(--tc-max-width);
  margin: 0 auto;
}

.heading {
  font-size: var(--tc-text-h2);
  font-weight: 700;
  color: var(--tc-text-primary);
  text-align: center;
  margin-bottom: var(--tc-space-xl);
}

.steps {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: var(--tc-space-xl);
  list-style: none;
}

.step {
  text-align: center;
  padding: var(--tc-space-lg);
  background: var(--tc-surface);
  border: 1px solid var(--tc-border);
  border-radius: var(--tc-radius-md);
}

.icon {
  font-size: 2.5rem;
  display: block;
  margin-bottom: var(--tc-space-md);
}

.title {
  font-size: var(--tc-text-h3);
  font-weight: 600;
  color: var(--tc-text-primary);
  margin-bottom: var(--tc-space-sm);
}

.description {
  font-size: var(--tc-text-body);
  color: var(--tc-text-secondary);
}

@media (max-width: 639px) {
  .steps {
    grid-template-columns: 1fr;
  }
}
```

- [ ] **Step 2: Verify build**

```bash
cd /Users/christy/Dev/town-crier/web && npm run build && npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add web/src/components/HowItWorks/
git commit -m "Add HowItWorks section with three-step flow"
```

---

### Task 8: CommunityGroups Component

**Files:**
- Create: `web/src/components/CommunityGroups/CommunityGroups.tsx`
- Create: `web/src/components/CommunityGroups/CommunityGroups.module.css`

- [ ] **Step 1: Create CommunityGroups component**

Highlight section differentiating Town Crier from a simple notification tool.

```tsx
// web/src/components/CommunityGroups/CommunityGroups.tsx
import styles from "./CommunityGroups.module.css";

const FEATURES = [
  {
    title: "Create a group",
    description:
      "Set up a community group for your street, estate, or neighbourhood association.",
  },
  {
    title: "Invite neighbours",
    description:
      "Share your group with neighbours so everyone stays informed about local planning.",
  },
  {
    title: "Coordinate responses",
    description:
      "Discuss applications and submit coordinated responses during consultation periods.",
  },
] as const;

export function CommunityGroups() {
  return (
    <section className={styles.section}>
      <div className={styles.inner}>
        <h2 className={styles.heading}>Stronger together</h2>
        <p className={styles.subheading}>
          Town Crier isn&rsquo;t just notifications &mdash; it&rsquo;s a tool
          for community action. Create groups with shared watch zones and
          coordinate with your neighbours.
        </p>
        <div className={styles.features}>
          {FEATURES.map((feature) => (
            <div key={feature.title} className={styles.feature}>
              <h3 className={styles.featureTitle}>{feature.title}</h3>
              <p className={styles.featureDescription}>
                {feature.description}
              </p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
```

```css
/* web/src/components/CommunityGroups/CommunityGroups.module.css */
.section {
  padding: var(--tc-space-xxl) var(--tc-space-md);
  background: var(--tc-surface);
}

.inner {
  max-width: var(--tc-max-width);
  margin: 0 auto;
}

.heading {
  font-size: var(--tc-text-h2);
  font-weight: 700;
  color: var(--tc-text-primary);
  text-align: center;
  margin-bottom: var(--tc-space-md);
}

.subheading {
  font-size: var(--tc-text-body);
  color: var(--tc-text-secondary);
  text-align: center;
  max-width: 640px;
  margin: 0 auto var(--tc-space-xl);
  line-height: 1.6;
}

.features {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: var(--tc-space-lg);
}

.feature {
  padding: var(--tc-space-lg);
  background: var(--tc-background);
  border: 1px solid var(--tc-border);
  border-radius: var(--tc-radius-md);
}

.featureTitle {
  font-size: var(--tc-text-h3);
  font-weight: 600;
  color: var(--tc-amber);
  margin-bottom: var(--tc-space-sm);
}

.featureDescription {
  font-size: var(--tc-text-body);
  color: var(--tc-text-secondary);
  line-height: 1.5;
}

@media (max-width: 639px) {
  .features {
    grid-template-columns: 1fr;
  }
}
```

- [ ] **Step 2: Verify build**

```bash
cd /Users/christy/Dev/town-crier/web && npm run build && npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add web/src/components/CommunityGroups/
git commit -m "Add CommunityGroups section highlighting shared watch zones"
```

---

### Task 9: Pricing Component

**Files:**
- Create: `web/src/components/Pricing/Pricing.tsx`
- Create: `web/src/components/Pricing/Pricing.module.css`

- [ ] **Step 1: Create Pricing component**

Three-tier pricing table with Personal highlighted as recommended.

```tsx
// web/src/components/Pricing/Pricing.tsx
import styles from "./Pricing.module.css";

const FEATURE_LABELS = [
  "Watch Zones",
  "Radius",
  "Notifications",
  "Search",
  "Historical data",
] as const;

type FeatureLabel = (typeof FEATURE_LABELS)[number];

interface Tier {
  name: string;
  price: string;
  period: string;
  recommended: boolean;
  features: Record<FeatureLabel, string>;
  trial: string | null;
}

const TIERS: Tier[] = [
  {
    name: "Free",
    price: "£0",
    period: "",
    recommended: false,
    features: {
      "Watch Zones": "1",
      Radius: "1 km",
      Notifications: "5/month",
      Search: "Browse only",
      "Historical data": "Forward only",
    },
    trial: null,
  },
  {
    name: "Personal",
    price: "£1.99",
    period: "/month",
    recommended: true,
    features: {
      "Watch Zones": "1",
      Radius: "5 km",
      Notifications: "Unlimited",
      Search: "Browse + filter",
      "Historical data": "Instant backfill",
    },
    trial: "7-day free trial",
  },
  {
    name: "Pro",
    price: "£5.99",
    period: "/month",
    recommended: false,
    features: {
      "Watch Zones": "Unlimited",
      Radius: "10 km",
      Notifications: "Unlimited",
      Search: "Full-text search",
      "Historical data": "Instant backfill",
    },
    trial: null,
  },
];

export function Pricing() {
  return (
    <section id="pricing" className={styles.section}>
      <div className={styles.inner}>
        <h2 className={styles.heading}>Simple, transparent pricing</h2>
        <div className={styles.grid}>
          {TIERS.map((tier) => (
            <div
              key={tier.name}
              className={`${styles.card} ${tier.recommended ? styles.recommended : ""}`}
            >
              {tier.recommended && (
                <span className={styles.badge}>Recommended</span>
              )}
              <h3 className={styles.tierName}>{tier.name}</h3>
              <div className={styles.price}>
                <span className={styles.priceValue}>{tier.price}</span>
                {tier.period && (
                  <span className={styles.pricePeriod}>{tier.period}</span>
                )}
              </div>
              {tier.trial && <p className={styles.trial}>{tier.trial}</p>}
              <ul className={styles.featureList}>
                {FEATURE_LABELS.map((label) => (
                  <li key={label} className={styles.featureItem}>
                    <span className={styles.featureLabel}>{label}</span>
                    <span className={styles.featureValue}>
                      {tier.features[label]}
                    </span>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
```

```css
/* web/src/components/Pricing/Pricing.module.css */
.section {
  padding: var(--tc-space-xxl) var(--tc-space-md);
}

.inner {
  max-width: var(--tc-max-width);
  margin: 0 auto;
}

.heading {
  font-size: var(--tc-text-h2);
  font-weight: 700;
  color: var(--tc-text-primary);
  text-align: center;
  margin-bottom: var(--tc-space-xl);
}

.grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: var(--tc-space-lg);
  align-items: start;
}

.card {
  background: var(--tc-surface);
  border: 1px solid var(--tc-border);
  border-radius: var(--tc-radius-md);
  padding: var(--tc-space-lg);
  position: relative;
}

.recommended {
  border-color: var(--tc-amber);
  box-shadow: 0 0 0 1px var(--tc-amber);
}

.badge {
  position: absolute;
  top: calc(-1 * var(--tc-space-sm));
  left: 50%;
  transform: translateX(-50%);
  background: var(--tc-amber);
  color: var(--tc-text-on-accent);
  font-size: var(--tc-text-small);
  font-weight: 600;
  padding: var(--tc-space-xs) var(--tc-space-md);
  border-radius: var(--tc-radius-full);
  white-space: nowrap;
}

.tierName {
  font-size: var(--tc-text-h3);
  font-weight: 600;
  color: var(--tc-text-primary);
  margin-bottom: var(--tc-space-sm);
  text-align: center;
}

.price {
  text-align: center;
  margin-bottom: var(--tc-space-md);
}

.priceValue {
  font-size: var(--tc-text-h2);
  font-weight: 700;
  color: var(--tc-amber);
}

.pricePeriod {
  font-size: var(--tc-text-small);
  color: var(--tc-text-secondary);
}

.trial {
  text-align: center;
  font-size: var(--tc-text-small);
  color: var(--tc-amber);
  margin-bottom: var(--tc-space-md);
}

.featureList {
  list-style: none;
}

.featureItem {
  display: flex;
  justify-content: space-between;
  padding: var(--tc-space-sm) 0;
  border-bottom: 1px solid var(--tc-border);
  font-size: var(--tc-text-small);
}

.featureItem:last-child {
  border-bottom: none;
}

.featureLabel {
  color: var(--tc-text-secondary);
}

.featureValue {
  color: var(--tc-text-primary);
  font-weight: 500;
}

@media (max-width: 639px) {
  .grid {
    grid-template-columns: 1fr;
    max-width: 400px;
    margin: 0 auto;
  }
}
```

- [ ] **Step 2: Verify build**

```bash
cd /Users/christy/Dev/town-crier/web && npm run build && npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add web/src/components/Pricing/
git commit -m "Add Pricing section with three-tier comparison table"
```

---

### Task 10: FAQ Component

**Files:**
- Create: `web/src/components/Faq/Faq.tsx`
- Create: `web/src/components/Faq/Faq.module.css`

- [ ] **Step 1: Create FAQ component**

Accordion-style FAQ using minimal React state. Uses `<details>`/`<summary>` for native accordion behaviour with CSS styling.

```tsx
// web/src/components/Faq/Faq.tsx
import styles from "./Faq.module.css";

const QUESTIONS = [
  {
    question: "Where does the data come from?",
    answer:
      "Town Crier sources its data from PlanIt, which aggregates planning applications directly from official local authority planning registers. The data is the same information published by your local council — we just make it easier to find and follow.",
  },
  {
    question: "Which areas are covered?",
    answer:
      "We currently monitor 417 local planning authorities across England, Scotland, and Wales. This covers the vast majority of planning decisions in Great Britain.",
  },
  {
    question: "Is it really free?",
    answer:
      "Yes. The free tier gives you 1 watch zone with a 1 km radius and up to 5 notifications per month — no credit card required. Paid plans unlock wider zones, unlimited notifications, and advanced search.",
  },
  {
    question: "Can I use it with my neighbours?",
    answer:
      "Absolutely. Community Groups let you create a shared watch zone with neighbours, so everyone in your group gets notified about the same planning applications. Great for resident associations and neighbourhood groups.",
  },
  {
    question: "How quickly will I be notified?",
    answer:
      "Planning applications are checked every 15 minutes. You'll typically receive a notification within minutes of a new application being published by your local authority.",
  },
] as const;

export function Faq() {
  return (
    <section id="faq" className={styles.section}>
      <div className={styles.inner}>
        <h2 className={styles.heading}>Frequently asked questions</h2>
        <div className={styles.list}>
          {QUESTIONS.map((item) => (
            <details key={item.question} className={styles.item}>
              <summary className={styles.question}>{item.question}</summary>
              <p className={styles.answer}>{item.answer}</p>
            </details>
          ))}
        </div>
      </div>
    </section>
  );
}
```

```css
/* web/src/components/Faq/Faq.module.css */
.section {
  padding: var(--tc-space-xxl) var(--tc-space-md);
  background: var(--tc-surface);
}

.inner {
  max-width: 720px;
  margin: 0 auto;
}

.heading {
  font-size: var(--tc-text-h2);
  font-weight: 700;
  color: var(--tc-text-primary);
  text-align: center;
  margin-bottom: var(--tc-space-xl);
}

.list {
  display: flex;
  flex-direction: column;
  gap: var(--tc-space-sm);
}

.item {
  background: var(--tc-background);
  border: 1px solid var(--tc-border);
  border-radius: var(--tc-radius-md);
  overflow: hidden;
}

.question {
  padding: var(--tc-space-md) var(--tc-space-lg);
  font-size: var(--tc-text-body);
  font-weight: 600;
  color: var(--tc-text-primary);
  cursor: pointer;
  list-style: none;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.question::after {
  content: "+";
  font-size: var(--tc-text-h3);
  color: var(--tc-amber);
  transition: transform 0.2s ease;
}

.item[open] .question::after {
  content: "−";
}

.question::-webkit-details-marker {
  display: none;
}

.answer {
  padding: 0 var(--tc-space-lg) var(--tc-space-md);
  font-size: var(--tc-text-body);
  color: var(--tc-text-secondary);
  line-height: 1.6;
}
```

- [ ] **Step 2: Verify build**

```bash
cd /Users/christy/Dev/town-crier/web && npm run build && npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add web/src/components/Faq/
git commit -m "Add FAQ section with native details/summary accordion"
```

---

### Task 11: Footer Component

**Files:**
- Create: `web/src/components/Footer/Footer.tsx`
- Create: `web/src/components/Footer/Footer.module.css`

- [ ] **Step 1: Create Footer component**

Final CTA with App Store badge, copyright, and legal links.

```tsx
// web/src/components/Footer/Footer.tsx
import styles from "./Footer.module.css";

const APP_STORE_URL = "https://apps.apple.com/app/town-crier/id000000000"; // TODO: Replace with real App Store URL

export function Footer() {
  const currentYear = new Date().getFullYear();

  return (
    <footer className={styles.footer}>
      <div className={styles.inner}>
        <div className={styles.ctaBlock}>
          <h2 className={styles.ctaHeading}>
            Your neighbourhood is changing. Stay informed.
          </h2>
          <a
            href={APP_STORE_URL}
            className={styles.cta}
            target="_blank"
            rel="noopener noreferrer"
            aria-label="Download Town Crier on the App Store"
          >
            Download on the App Store
          </a>
        </div>
        <div className={styles.legal}>
          <p className={styles.copyright}>
            &copy; {currentYear} Town Crier. All rights reserved.
          </p>
          <nav className={styles.legalLinks} aria-label="Legal">
            <a href="/privacy" className={styles.legalLink}>
              Privacy Policy
            </a>
            <a href="/terms" className={styles.legalLink}>
              Terms of Service
            </a>
          </nav>
        </div>
      </div>
    </footer>
  );
}
```

```css
/* web/src/components/Footer/Footer.module.css */
.footer {
  padding: var(--tc-space-xxl) var(--tc-space-md);
  border-top: 1px solid var(--tc-border);
}

.inner {
  max-width: var(--tc-max-width);
  margin: 0 auto;
}

.ctaBlock {
  text-align: center;
  margin-bottom: var(--tc-space-xxl);
}

.ctaHeading {
  font-size: var(--tc-text-h2);
  font-weight: 700;
  color: var(--tc-text-primary);
  margin-bottom: var(--tc-space-lg);
}

.cta {
  display: inline-block;
  font-size: var(--tc-text-body);
  font-weight: 600;
  color: var(--tc-text-on-accent);
  background-color: var(--tc-amber);
  padding: var(--tc-space-sm) var(--tc-space-xl);
  border-radius: var(--tc-radius-md);
  text-decoration: none;
  transition: background-color 0.2s ease;
}

.cta:hover {
  background-color: var(--tc-amber-hover);
  color: var(--tc-text-on-accent);
}

.legal {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-top: var(--tc-space-lg);
  border-top: 1px solid var(--tc-border);
}

.copyright {
  font-size: var(--tc-text-small);
  color: var(--tc-text-tertiary);
}

.legalLinks {
  display: flex;
  gap: var(--tc-space-lg);
}

.legalLink {
  font-size: var(--tc-text-small);
  color: var(--tc-text-secondary);
  text-decoration: none;
}

.legalLink:hover {
  color: var(--tc-text-primary);
}

@media (max-width: 639px) {
  .legal {
    flex-direction: column;
    gap: var(--tc-space-md);
    text-align: center;
  }
}
```

- [ ] **Step 2: Verify build**

```bash
cd /Users/christy/Dev/town-crier/web && npm run build && npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add web/src/components/Footer/
git commit -m "Add Footer with final CTA and legal links"
```

---

### Task 12: Compose App.tsx

**Files:**
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Wire all sections into App.tsx**

```tsx
// web/src/App.tsx
import { useTheme } from "./hooks/useTheme";
import { Navbar } from "./components/Navbar/Navbar";
import { Hero } from "./components/Hero/Hero";
import { StatsBar } from "./components/StatsBar/StatsBar";
import { HowItWorks } from "./components/HowItWorks/HowItWorks";
import { CommunityGroups } from "./components/CommunityGroups/CommunityGroups";
import { Pricing } from "./components/Pricing/Pricing";
import { Faq } from "./components/Faq/Faq";
import { Footer } from "./components/Footer/Footer";

export function App() {
  const { theme, toggleTheme } = useTheme();

  return (
    <>
      <Navbar theme={theme} onThemeToggle={toggleTheme} />
      <main>
        <Hero />
        <StatsBar />
        <HowItWorks />
        <CommunityGroups />
        <Pricing />
        <Faq />
      </main>
      <Footer />
    </>
  );
}
```

- [ ] **Step 2: Verify full build**

```bash
cd /Users/christy/Dev/town-crier/web && npm run build && npx tsc --noEmit
```

Expected: Clean build. No TypeScript errors.

- [ ] **Step 3: Visual smoke test**

```bash
cd /Users/christy/Dev/town-crier/web && npm run dev
```

Open in browser. Verify:
- All sections render in order
- Theme toggle switches between light and dark
- Mobile hamburger menu works (resize browser below 640px)
- Anchor links scroll to correct sections
- Page uses Inter font

- [ ] **Step 4: Commit**

```bash
git add web/src/App.tsx
git commit -m "Compose all landing page sections in App.tsx"
```

---

### Task 13: Azure Static Web Apps Config

**Files:**
- Create: `web/staticwebapp.config.json`
- Create: `web/public/robots.txt`

- [ ] **Step 1: Create staticwebapp.config.json**

SPA fallback routing and security headers.

```json
{
  "navigationFallback": {
    "rewrite": "/index.html",
    "exclude": ["/assets/*", "/*.svg", "/*.png", "/*.ico"]
  },
  "globalHeaders": {
    "X-Content-Type-Options": "nosniff",
    "X-Frame-Options": "DENY",
    "Referrer-Policy": "strict-origin-when-cross-origin"
  },
  "mimeTypes": {
    ".json": "application/json"
  }
}
```

- [ ] **Step 2: Create robots.txt**

```
User-agent: *
Allow: /
```

- [ ] **Step 3: Commit**

```bash
git add web/staticwebapp.config.json web/public/robots.txt
git commit -m "Add Azure Static Web Apps config with SPA fallback and security headers"
```

---

### Task 14: GitHub Actions Workflow

**Files:**
- Create: `.github/workflows/web-ci.yml`

Reference the existing `api-ci.yml` for pattern consistency (path filters, concurrency groups, timeout).

- [ ] **Step 1: Create the workflow**

```yaml
# Requires secrets:
#   AZURE_STATIC_WEB_APPS_API_TOKEN — Deployment token from Azure Static Web Apps resource

name: Web CI

on:
  push:
    branches: [main]
    paths:
      - "web/**"
      - ".github/workflows/web-ci.yml"
  pull_request:
    branches: [main]
    paths:
      - "web/**"
      - ".github/workflows/web-ci.yml"

concurrency:
  group: web-ci-${{ github.ref }}
  cancel-in-progress: true

permissions:
  contents: read

jobs:
  build:
    name: Build & type-check
    runs-on: ubuntu-latest
    timeout-minutes: 5
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v4
        with:
          node-version: 22
          cache: "npm"
          cache-dependency-path: web/package-lock.json

      - name: Install dependencies
        run: npm ci
        working-directory: web

      - name: Type check
        run: npx tsc --noEmit
        working-directory: web

      - name: Build
        run: npm run build
        working-directory: web

  deploy:
    name: Deploy to Azure Static Web Apps
    needs: [build]
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    timeout-minutes: 5
    permissions:
      contents: read
      id-token: write
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v4
        with:
          node-version: 22
          cache: "npm"
          cache-dependency-path: web/package-lock.json

      - name: Install dependencies
        run: npm ci
        working-directory: web

      - name: Build
        run: npm run build
        working-directory: web

      - name: Deploy
        uses: Azure/static-web-apps-deploy@v1
        with:
          azure_static_web_apps_api_token: ${{ secrets.AZURE_STATIC_WEB_APPS_API_TOKEN }}
          action: "upload"
          app_location: "web"
          output_location: "dist"
          skip_app_build: true

  deploy-preview:
    name: Deploy PR preview
    if: github.event_name == 'pull_request'
    runs-on: ubuntu-latest
    timeout-minutes: 5
    permissions:
      contents: read
      pull-requests: write
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v4
        with:
          node-version: 22
          cache: "npm"
          cache-dependency-path: web/package-lock.json

      - name: Install dependencies
        run: npm ci
        working-directory: web

      - name: Build
        run: npm run build
        working-directory: web

      - name: Deploy preview
        uses: Azure/static-web-apps-deploy@v1
        with:
          azure_static_web_apps_api_token: ${{ secrets.AZURE_STATIC_WEB_APPS_API_TOKEN }}
          action: "upload"
          app_location: "web"
          output_location: "dist"
          skip_app_build: true
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/web-ci.yml
git commit -m "Add GitHub Actions workflow for web CI/CD with Azure Static Web Apps"
```

---

### Task 15: Pulumi Infrastructure

**Files:**
- Modify: `infra/Program.cs`
- Modify: `infra/town-crier.infra.csproj` (if new NuGet package needed)

Reference: Existing infra at `infra/Program.cs` uses `Pulumi.AzureNative.*` with environment-based naming and shared tags.

- [ ] **Step 1: Check if `Pulumi.AzureNative.Web` is already available**

The Azure Static Web Apps resource lives in `Pulumi.AzureNative.Web`. Check if the existing `Pulumi.AzureNative` meta-package includes it:

```bash
cd /Users/christy/Dev/town-crier/infra && grep -i "AzureNative" town-crier.infra.csproj
```

If only the meta-package `Pulumi.AzureNative` is referenced, it already includes `Web`. No additional package needed.

- [ ] **Step 2: Add Static Web App resource to Program.cs**

Add the following after the Container App resource (around line 339), before the `return` statement:

```csharp
using Pulumi.AzureNative.Web;
using Pulumi.AzureNative.Web.Inputs;
```

Add to the top `using` block. Then add the resource:

```csharp
// Azure Static Web App (Landing Page)
var staticWebApp = new StaticSite($"swa-town-crier-{env}", new StaticSiteArgs
{
    Name = $"swa-town-crier-{env}",
    ResourceGroupName = resourceGroup.Name,
    Location = "westeurope",
    Sku = new Pulumi.AzureNative.Web.Inputs.SkuDescriptionArgs
    {
        Name = "Free",
        Tier = "Free",
    },
    Tags = tags,
});
```

Add outputs to the return dictionary:

```csharp
["staticWebAppUrl"] = staticWebApp.DefaultHostname.Apply(h => $"https://{h}"),
["staticWebAppName"] = staticWebApp.Name,
```

Note: Custom domain binding for `towncrierapp.uk` will be done manually in Azure Portal or via a subsequent Pulumi update once DNS is configured, as it requires DNS verification that can't be automated in a single step.

- [ ] **Step 3: Verify Pulumi project builds**

```bash
cd /Users/christy/Dev/town-crier/infra && dotnet build
```

Expected: Build succeeds.

- [ ] **Step 4: Commit**

```bash
git add infra/Program.cs
git commit -m "Add Azure Static Web App resource for landing page hosting"
```

---

### Task 16: Final Verification

- [ ] **Step 1: Full build from clean state**

```bash
cd /Users/christy/Dev/town-crier/web && rm -rf node_modules dist && npm install && npm run build && npx tsc --noEmit
```

Expected: Clean install, clean build, no type errors.

- [ ] **Step 2: Infra build**

```bash
cd /Users/christy/Dev/town-crier/infra && dotnet build
```

Expected: Clean build.

- [ ] **Step 3: Check no uncommitted changes**

```bash
git status
```

Expected: Working tree clean (or only expected untracked files like `.beads/`).
