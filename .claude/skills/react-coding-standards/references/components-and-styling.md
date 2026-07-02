# Components & Styling (reference)

Read when building or reviewing a component (structure, semantic HTML, accessibility, list keys) or writing CSS Modules, responsive layouts, or design-token styles. The core (`SKILL.md`) states the rules and forbidden patterns; this file is the full prose and examples.

## 3. Component Patterns

React components follow a consistent structure that keeps them small, testable, and accessible.

- **Function Components Only:** No class components. Function components with hooks compose better and are the standard React pattern.
- **Named Exports:** Use named exports (`export function Navbar()`) not default exports. Named exports enable better IDE refactoring and prevent inconsistent import names.
- **Props Interfaces:** Define props as an `interface` directly above the component. Name it `Props` for file-local use, or `{ComponentName}Props` if exported.
- **Semantic HTML:** Use the correct element for its purpose. `<button>` for actions, `<a>` for navigation, `<nav>` for navigation bars, `<section>` for page sections, `<ul>`/`<li>` for lists. Never use `<div onClick>` for clickable elements — it breaks keyboard navigation and screen readers.
- **Accessibility:** All interactive elements must be keyboard-accessible. Images need `alt` text (empty `alt=""` for decorative images). Use `aria-label` when visual context isn't available to screen readers. Status indicators must pair color with an icon or text label (per design-language skill).
- **No Inline Styles:** Use CSS Modules for all styling. Inline `style={}` props bypass the design token system and make themes impossible.
- **`key` Props:** Use stable, unique identifiers (domain IDs) for list keys. Never use array index as key unless the list is static and never reordered.

**Example — Component Structure:**
```tsx
// components/StatusBadge/StatusBadge.tsx
import type { ApplicationStatus } from "../../domain/entities/planning-application";
import styles from "./StatusBadge.module.css";

interface Props {
  status: ApplicationStatus;
}

export function StatusBadge({ status }: Props) {
  return (
    <span className={`${styles.badge} ${styles[status]}`} role="status">
      <StatusIcon status={status} />
      {formatStatus(status)}
    </span>
  );
}

const STATUS_LABELS: Record<ApplicationStatus, string> = {
  "under-review": "Under Review",
  approved: "Approved",
  refused: "Refused",
  withdrawn: "Withdrawn",
  appealed: "Appealed",
};

function formatStatus(status: ApplicationStatus): string {
  return STATUS_LABELS[status];
}
```

## 4. Styling (CSS Modules + Design Tokens)

CSS Modules provide scoped styles with zero runtime cost — each `.module.css` file generates unique class names at build time, so styles never leak between components. Design tokens (CSS custom properties) in `tokens.css` ensure visual consistency with the design-language skill.

- **CSS Modules for all component styles.** One `.module.css` file per component, co-located in the component's directory.
- **Design tokens for all visual values.** Never hard-code colors, spacing, font sizes, or border radii. Always reference `var(--tc-*)` tokens from `tokens.css`. This is what makes dark/light theme switching work — the tokens change value and every component updates automatically.
- **Responsive design with mobile-first breakpoints.** Base styles target mobile. `@media` queries add tablet (640px) and desktop (1024px) layouts.
- **No CSS-in-JS.** No styled-components, Emotion, or Tailwind. CSS Modules give scoping without runtime overhead or build complexity.
- **Class composition:** Use template literals for conditional classes. No need for the `classnames` npm package for simple cases.

**Example — CSS Module:**
```css
/* components/Pricing/Pricing.module.css */
.container {
  max-width: 1120px;
  margin: 0 auto;
  padding: var(--tc-space-xxl) var(--tc-space-md);
}

.heading {
  font-size: var(--tc-text-h2);
  color: var(--tc-text-primary);
  text-align: center;
  margin-bottom: var(--tc-space-xl);
}

.grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: var(--tc-space-lg);
}

@media (min-width: 640px) {
  .grid {
    grid-template-columns: repeat(3, 1fr);
  }
}

.card {
  background: var(--tc-surface);
  border: 1px solid var(--tc-border);
  border-radius: var(--tc-radius-md);
  padding: var(--tc-space-lg);
}

.recommended {
  border-color: var(--tc-amber);
  box-shadow: 0 0 0 1px var(--tc-amber);
}
```
