# Architecture & Domain (reference)

Read when scaffolding the directory layout or a feature module, shaping domain entities or value objects (branded types), or reasoning about Clean Architecture layering, hooks-as-ViewModel, and the cross-platform mapping. The core (`SKILL.md`) states the rules; this file is the full prose and examples.

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

## 1. Domain Purity

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

## 2. Architecture Style (Clean / Feature-Sliced)

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
