---
name: react-coding-standards
description: "MUST consult before writing ANY React, TypeScript, CSS, or frontend code in /web — including fixing bugs, updating styles, adding features, or configuring build tools. This skill defines how ALL web frontend code should be written in this project. Trigger on ANY of these: creating or modifying React components, JSX, TSX, or .module.css files; writing hooks (useAnything); fixing TypeScript errors in web code; editing CSS Modules or responsive layouts; creating domain types, API clients, or value objects for the web; writing Vitest tests, spies, or fixtures; configuring vite.config.ts, tsconfig.json, or eslint; scaffolding features, pages, or shared components; building landing page sections (Hero, Navbar, Pricing, FAQ, Footer, etc.); reviewing PRs that touch /web. If the work involves the /web directory, a .tsx file, a .module.css file, a React hook, Vite, or any browser-facing TypeScript — use this skill. It contains project-specific architectural patterns (hook-as-ViewModel, branded types, CSS Module token conventions, feature-sliced directory structure, hand-written test doubles) that differ significantly from generic React. Do NOT use for: C#/.NET API code, iOS/Swift, Pulumi infrastructure, GitHub Actions CI/CD, or non-web code."
---

# React Coding Standards

## Overview

This skill provides guidelines for React/TypeScript development in the Town Crier web app, enforcing **Clean Architecture**, **Domain Purity**, and **Test-Driven Development**. The web app starts as a static landing page and grows toward feature parity with the iOS app, so these standards establish patterns that scale from day one.

The React architecture intentionally mirrors the iOS (MVVM-C) and .NET (Hexagonal/CQRS) approaches used elsewhere in the monorepo. Custom hooks serve as the React equivalent of iOS ViewModels — they own state and orchestration logic while components remain passive renderers. This consistency means domain concepts, API contracts, and architectural reasoning transfer across all three platforms.

## Project Structure

The web app lives in `/web`. The structure evolves as features are added, but the architectural layers are established from the start.

### Current (Landing Page)

```
/web
├── index.html
├── vite.config.ts
├── tsconfig.json
├── package.json
├── staticwebapp.config.json
├── src/
│   ├── main.tsx                    # React entry point
│   ├── App.tsx                     # Root component
│   ├── components/                 # Landing page sections + shared UI
│   │   ├── Navbar/
│   │   │   ├── Navbar.tsx
│   │   │   └── Navbar.module.css
│   │   ├── Hero/
│   │   ├── Pricing/
│   │   └── ...
│   ├── styles/
│   │   ├── tokens.css              # Design system CSS custom properties
│   │   └── global.css              # Reset, base typography
│   └── assets/
└── public/
```

### Future (Full Web App)

As features from the iOS app are ported to web, the structure grows into a feature-sliced architecture:

```
/web/src/
├── domain/                         # Pure TypeScript — no React imports
│   ├── entities/                   # PlanningApplication, AlertSubscription, etc.
│   ├── value-objects/              # Postcode, LocalAuthority
│   ├── ports/                      # Repository interfaces (TypeScript)
│   └── errors.ts                   # Typed domain errors
├── data/                           # API clients, repository implementations
│   ├── api/                        # HTTP client, endpoint definitions
│   └── repositories/               # Concrete implementations of domain ports
├── features/                       # Feature-sliced modules
│   ├── landing/                    # Landing page (promoted from components/)
│   ├── application-feed/           # Browse planning applications
│   │   ├── ApplicationFeed.tsx     # Passive view component
│   │   ├── ApplicationFeed.module.css
│   │   ├── useApplicationFeed.ts   # Hook (ViewModel equivalent)
│   │   └── components/             # Feature-private sub-components
│   └── application-detail/
├── components/                     # Shared UI components (Button, Card, StatusBadge)
├── styles/                         # tokens.css, global.css
└── assets/
```

Directory names use kebab-case. TypeScript files use PascalCase for components (`Navbar.tsx`), camelCase for hooks and utilities (`useApplicationFeed.ts`).

## Cross-Platform Architecture Mapping

The same business domain spans all three platforms. Understanding the mapping helps apply consistent patterns:

| Concept | .NET API | iOS App | React Web |
|---------|----------|---------|-----------|
| Business logic | Domain entity methods | Domain struct methods | Domain type functions |
| Orchestration | Command/Query Handler | ViewModel | Custom hook |
| Presentation | Controller endpoint | SwiftUI View | React component |
| Navigation | URL routing | Coordinator | React Router |
| Data port | `I*Repository` interface | `*Repository` protocol | `*Repository` interface |
| Data adapter | `Cosmos*Repository` class | `SwiftData*Repository` | `Api*Repository` class |
| Test target | Handler | ViewModel | Custom hook |
| Test doubles | Hand-written Fake/Spy | Hand-written Spy | Hand-written Spy |
| Test data | Builder pattern | Static extension fixtures | Factory functions |

## Core Mandates

### 1. Domain Purity

The domain layer is plain TypeScript with zero framework dependencies. No React, no browser APIs, no `fetch`. This keeps business logic testable, portable, and shared across features — mirroring the iOS approach where `town-crier-domain` imports no Apple frameworks.

- **Pure TypeScript:** The `domain/` directory must not import from React, browser APIs (`fetch`, `localStorage`, `window`), or any npm package. `import type` from external packages is allowed for shared API contract types.
- **Branded Types:** Use branded types for IDs and value objects to prevent mixing up strings that represent different things. TypeScript's structural typing means a plain `string` for a postcode and a plain `string` for a reference are interchangeable — branded types fix this.
- **Validation at the boundary:** Domain types enforce their own invariants via factory functions that validate input and throw typed errors.
- **No `any`:** Use `unknown` when the type is genuinely not known, then narrow with type guards. `any` silently disables TypeScript's safety.

**Example — Branded Type (Value Object):**
```typescript
// domain/value-objects/postcode.ts
type PostcodeBrand = { readonly __brand: "Postcode" };
export type Postcode = string & PostcodeBrand;

const POSTCODE_PATTERN = /^[A-Z]{1,2}\d[A-Z\d]?\s?\d[A-Z]{2}$/;

export function createPostcode(raw: string): Postcode {
  const trimmed = raw.trim().toUpperCase();
  if (!POSTCODE_PATTERN.test(trimmed)) {
    throw new DomainError(`Invalid postcode: ${raw}`);
  }
  return trimmed as Postcode;
}
```

**Example — Domain Entity:**
```typescript
// domain/entities/planning-application.ts
export interface PlanningApplication {
  readonly id: PlanningApplicationId;
  readonly reference: ApplicationReference;
  readonly authority: LocalAuthority;
  readonly status: ApplicationStatus;
  readonly receivedDate: Date;
  readonly description: string;
  readonly address: string;
}

export type ApplicationStatus =
  | "under-review"
  | "approved"
  | "refused"
  | "withdrawn"
  | "appealed";

export function canBeDecided(app: PlanningApplication): boolean {
  return app.status === "under-review";
}
```

### 2. Architecture Style (Clean / Feature-Sliced)

Clean Architecture separates concerns so business rules don't depend on UI framework choices, and UI choices don't depend on data-layer choices. Features are organised as self-contained modules — each feature owns its components, hooks, and styles.

- **Dependency Rule:** Dependencies flow inward. Components depend on hooks. Hooks depend on domain ports (repository interfaces). Neither components nor hooks know about concrete API implementations. The composition root (`App.tsx` or a provider) wires concrete implementations.
- **Hooks as ViewModels:** Custom hooks are the React equivalent of iOS ViewModels. They own state, call repository methods, and expose state + actions to components. Components should not contain `fetch` calls, complex state transitions, or business logic.
- **Passive Components:** Components render state from hooks and forward user events. Conditional rendering based on hook state (loading, error, empty) is fine, but domain decisions (is this application eligible for appeal?) belong in the domain layer or hook.
- **Feature Isolation:** Features must not import from each other. Shared types live in `domain/`, shared UI in `components/`. If two features need the same data, they both depend on the same domain port, not on each other.

**Example — Custom Hook (ViewModel equivalent):**
```typescript
// features/application-feed/useApplicationFeed.ts
import { useState, useEffect, useCallback } from "react";
import type { PlanningApplication } from "../../domain/entities/planning-application";
import type { PlanningApplicationRepository } from "../../domain/ports/planning-application-repository";
import type { LocalAuthority } from "../../domain/value-objects/local-authority";
import type { DomainError } from "../../domain/errors";

interface ApplicationFeedState {
  applications: PlanningApplication[];
  isLoading: boolean;
  error: DomainError | null;
}

export function useApplicationFeed(
  repository: PlanningApplicationRepository,
  authority: LocalAuthority
) {
  const [state, setState] = useState<ApplicationFeedState>({
    applications: [],
    isLoading: false,
    error: null,
  });

  const loadApplications = useCallback(async () => {
    setState(prev => ({ ...prev, isLoading: true, error: null }));
    try {
      const applications = await repository.fetchApplications(authority);
      setState({ applications, isLoading: false, error: null });
    } catch (err) {
      setState(prev => ({
        ...prev,
        isLoading: false,
        error: err as DomainError,
      }));
    }
  }, [repository, authority]);

  useEffect(() => {
    loadApplications();
  }, [loadApplications]);

  return { ...state, refresh: loadApplications };
}
```

**Example — Passive Component:**
```tsx
// features/application-feed/ApplicationFeed.tsx
import type { PlanningApplicationRepository } from "../../domain/ports/planning-application-repository";
import type { LocalAuthority } from "../../domain/value-objects/local-authority";
import { useApplicationFeed } from "./useApplicationFeed";
import { ApplicationRow } from "./components/ApplicationRow";
import styles from "./ApplicationFeed.module.css";

interface Props {
  repository: PlanningApplicationRepository;
  authority: LocalAuthority;
}

export function ApplicationFeed({ repository, authority }: Props) {
  const { applications, isLoading, error } = useApplicationFeed(repository, authority);

  if (isLoading) return <div className={styles.loading}>Loading...</div>;
  if (error) return <div className={styles.error}>{error.message}</div>;

  return (
    <ul className={styles.list}>
      {applications.map(app => (
        <ApplicationRow key={app.id} application={app} />
      ))}
    </ul>
  );
}
```

### 3. Component Patterns

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

### 4. Styling (CSS Modules + Design Tokens)

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

### 5. Testing Strategy (Vitest + Testing Library)

Testing is deferred for the static landing page (there's nothing to test beyond "does it render"), but these standards apply the moment interactive features arrive. The approach mirrors the .NET and iOS testing strategies: test behavior through the orchestration layer (hooks), not implementation details.

- **Framework:** Vitest (integrates with Vite) + React Testing Library. Vitest provides the test runner and assertions; Testing Library provides DOM queries that encourage accessible, behavioral testing.
- **Unit of Work:** Custom hooks are the primary test target — they contain the orchestration logic, just like Handlers (.NET) and ViewModels (iOS). Domain entities with business rules also warrant direct unit tests. Components are tested via integration tests that render them with real hooks.
- **Workflow:** Red-Green-Refactor. Write the test first.
- **Manual Test Doubles:** Hand-written spies and fakes, consistent with the no-mocking-frameworks policy across the monorepo. No `vi.fn()` or `vi.mock()` for repository dependencies — write explicit spy classes that implement the port interface. This keeps tests readable and avoids magic.
- **Fixtures:** Factory functions that return domain entities with sensible defaults. Override specific fields via spread syntax. This is the TypeScript equivalent of the Builder pattern (.NET) and static extensions (iOS).

**Example — Repository Port (Domain layer):**
```typescript
// domain/ports/planning-application-repository.ts
export interface PlanningApplicationRepository {
  fetchApplications(authority: LocalAuthority): Promise<PlanningApplication[]>;
  fetchApplication(id: PlanningApplicationId): Promise<PlanningApplication>;
}
```

**Example — Spy (Test double):**
```typescript
// __tests__/spies/spy-planning-application-repository.ts
export class SpyPlanningApplicationRepository implements PlanningApplicationRepository {
  fetchApplicationsCalls: LocalAuthority[] = [];
  fetchApplicationsResult: PlanningApplication[] = [];

  async fetchApplications(authority: LocalAuthority): Promise<PlanningApplication[]> {
    this.fetchApplicationsCalls.push(authority);
    return this.fetchApplicationsResult;
  }

  fetchApplicationCalls: PlanningApplicationId[] = [];
  fetchApplicationResult: PlanningApplication = pendingReview();

  async fetchApplication(id: PlanningApplicationId): Promise<PlanningApplication> {
    this.fetchApplicationCalls.push(id);
    return this.fetchApplicationResult;
  }
}
```

**Example — Fixture (Factory function):**
```typescript
// __tests__/fixtures/planning-application.fixtures.ts
export function pendingReview(
  overrides?: Partial<PlanningApplication>
): PlanningApplication {
  return {
    id: "APP-001" as PlanningApplicationId,
    reference: "2026/0042" as ApplicationReference,
    authority: { code: "CAM", name: "Cambridge City Council" },
    status: "under-review",
    receivedDate: new Date("2026-01-15"),
    description: "Erection of two-storey rear extension",
    address: "12 Mill Road, Cambridge, CB1 2AD",
    ...overrides,
  };
}

export function approved(
  overrides?: Partial<PlanningApplication>
): PlanningApplication {
  return {
    ...pendingReview(),
    id: "APP-002" as PlanningApplicationId,
    reference: "2026/0099" as ApplicationReference,
    status: "approved",
    description: "Change of use from retail to residential",
    address: "45 High Street, Cambridge, CB2 1LA",
    ...overrides,
  };
}
```

**Example — Hook Test:**
```typescript
// features/application-feed/__tests__/useApplicationFeed.test.ts
import { renderHook, waitFor } from "@testing-library/react";
import { useApplicationFeed } from "../useApplicationFeed";
import { SpyPlanningApplicationRepository } from "./spies/spy-planning-application-repository";
import { pendingReview, approved } from "./fixtures/planning-application.fixtures";

describe("useApplicationFeed", () => {
  it("populates applications on successful fetch", async () => {
    const spy = new SpyPlanningApplicationRepository();
    spy.fetchApplicationsResult = [pendingReview(), approved()];

    const { result } = renderHook(() =>
      useApplicationFeed(spy, { code: "CAM", name: "Cambridge" })
    );

    await waitFor(() => {
      expect(result.current.applications).toHaveLength(2);
      expect(result.current.isLoading).toBe(false);
      expect(result.current.error).toBeNull();
    });
  });

  it("sets error on failed fetch", async () => {
    const spy = new SpyPlanningApplicationRepository();
    spy.fetchApplications = async () => {
      throw new DomainError("Network unavailable");
    };

    const { result } = renderHook(() =>
      useApplicationFeed(spy, { code: "CAM", name: "Cambridge" })
    );

    await waitFor(() => {
      expect(result.current.error?.message).toBe("Network unavailable");
      expect(result.current.applications).toEqual([]);
    });
  });
});
```

### 6. Data Access

Data access is deferred for the landing page (no API calls). When the web app starts calling the .NET API, these patterns apply — mirroring the repository pattern used in .NET and iOS.

- **Repository Pattern:** Define interfaces (ports) in `domain/ports/`. Implement them in `data/repositories/` using `fetch`. Components and hooks never call `fetch` directly.
- **API Client:** A shared `ApiClient` in `data/api/` handles base URL, auth headers, and error mapping. Repository implementations use it, centralising HTTP mechanics.
- **DTO Mapping:** API response shapes (DTOs) are separate types from domain entities. The repository maps between them, keeping domain types clean and decoupled from API changes.
- **Error Mapping:** API errors (HTTP status codes, network failures) are mapped to typed `DomainError` instances at the repository boundary. Hooks and components only deal with domain errors.

**Example — Repository Implementation (Adapter):**
```typescript
// data/repositories/api-planning-application-repository.ts
export class ApiPlanningApplicationRepository implements PlanningApplicationRepository {
  constructor(private readonly api: ApiClient) {}

  async fetchApplications(authority: LocalAuthority): Promise<PlanningApplication[]> {
    const dtos = await this.api.get<PlanningApplicationDto[]>(
      `/authorities/${authority.code}/applications`
    );
    return dtos.map(mapToDomain);
  }

  async fetchApplication(id: PlanningApplicationId): Promise<PlanningApplication> {
    const dto = await this.api.get<PlanningApplicationDto>(`/applications/${id}`);
    return mapToDomain(dto);
  }
}

function mapToDomain(dto: PlanningApplicationDto): PlanningApplication {
  return {
    id: dto.id as PlanningApplicationId,
    reference: dto.reference as ApplicationReference,
    authority: { code: dto.authorityCode, name: dto.authorityName },
    status: dto.status as ApplicationStatus,
    receivedDate: new Date(dto.receivedDate),
    description: dto.description,
    address: dto.address,
  };
}
```

## TypeScript Configuration

Strict TypeScript settings catch bugs at compile time that would otherwise surface as runtime errors. These settings are non-negotiable:

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

- `strict: true` enables all strict checks (strictNullChecks, strictFunctionTypes, etc.)
- `noUncheckedIndexedAccess` prevents silently accessing `undefined` from arrays and objects — forces handling the case where an index might not exist

## Workflow

### 1. Development

```bash
cd web && npm run dev      # Vite dev server with hot reload
cd web && npm run build    # Production build to /web/dist
```

### 2. Type Checking

```bash
cd web && npx tsc --noEmit   # Check types without emitting
```

### 3. Testing (when Vitest is added)

```bash
cd web && npx vitest           # Run tests in watch mode
cd web && npx vitest run       # Single run (CI)
```

### 4. Linting (when ESLint is added)

Copy the bundled ESLint config:

```bash
cp -f .claude/skills/react-coding-standards/assets/eslint.config.js ./web/
```

Then run:

```bash
cd web && npx eslint src/
```

## Guidelines

### Naming Conventions
- **Components:** PascalCase files and exports (`Navbar.tsx`, `StatusBadge.tsx`)
- **Hooks:** camelCase prefixed with `use` (`useApplicationFeed.ts`, `useTheme.ts`)
- **Domain types:** PascalCase for types/interfaces, camelCase for functions
- **CSS Module classes:** camelCase (`.container`, `.statsCard`, `.priceHighlighted`)
- **Directories:** kebab-case (`application-feed/`, `value-objects/`)
- **Constants:** UPPER_SNAKE_CASE (`MAX_WATCH_ZONES`, `API_BASE_URL`)
- **Event handlers:** `handle*` in the component, `on*` in props (`onClick` prop → `handleClick` handler)

### Best Practices
- **No `any`:** Use `unknown` and narrow with type guards.
- **Named exports only:** Default exports lead to inconsistent import names and weaker IDE support.
- **No barrel files (`index.ts`) re-exporting everything:** They break tree-shaking and create circular dependency risks. Import directly from the source file.
- **Prefer `interface` over `type` for object shapes:** Interfaces give better error messages and are extendable. Use `type` for unions, intersections, and mapped types.
- **No `useEffect` for derived state:** If a value can be computed from existing state or props, compute it during render. Use `useMemo` only if the computation is genuinely expensive.
- **No prop drilling beyond 2 levels:** If a prop passes through more than one intermediate component, use React context or restructure the component tree.
- **Immutable state updates:** Never mutate state directly. Use spread syntax or `structuredClone` for nested updates.
