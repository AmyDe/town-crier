---
name: react-coding-standards
description: "MUST consult before writing ANY React, TypeScript, CSS, or frontend code in /web — including fixing bugs, updating styles, adding features, or configuring build tools. This skill defines how ALL web frontend code should be written in this project. Trigger on ANY of these: creating or modifying React components, JSX, TSX, or .module.css files; writing hooks (useAnything); fixing TypeScript errors in web code; editing CSS Modules or responsive layouts; creating domain types, API clients, or value objects for the web; writing Vitest tests, spies, or fixtures; configuring vite.config.ts, tsconfig.json, or eslint; scaffolding features, pages, or shared components; building landing page sections (Hero, Navbar, Pricing, FAQ, Footer, etc.); reviewing PRs that touch /web. If the work involves the /web directory, a .tsx file, a .module.css file, a React hook, Vite, or any browser-facing TypeScript — use this skill. It contains project-specific architectural patterns (hook-as-ViewModel, branded types, CSS Module token conventions, feature-sliced directory structure, hand-written test doubles) that differ significantly from generic React. Do NOT use for: C#/.NET API code, iOS/Swift, Pulumi infrastructure, GitHub Actions CI/CD, or non-web code."
---

# React Coding Standards

React/TypeScript for the Town Crier web app (`/web`), enforcing **Clean Architecture**, **Domain Purity**, and **TDD**. The architecture intentionally mirrors iOS (MVVM-C) and the .NET API (Hexagonal/CQRS): custom hooks are the React equivalent of iOS ViewModels — they own state and orchestration; components stay passive renderers. Read this core first; pull the matching reference below when the bead touches that area.

## Architecture (always applies)

- **Feature-sliced layout under `/web/src`.** The landing page lives in `components/`; as iOS features port over, promote each to a self-contained `features/<name>/` module (passive `<Name>.tsx` view + `use<Name>.ts` hook + `<Name>.module.css` + feature-private `components/`). Shared domain in `domain/`, shared UI in `components/`, cross-cutting HTTP in `data/`. **Directories kebab-case**; component files/exports PascalCase; hooks camelCase `use*`.
- **Domain purity.** `domain/` is plain TypeScript with zero framework deps — no React, no browser APIs (`fetch`/`localStorage`/`window`), no npm package (only `import type` of shared API-contract types). Business rules live in the domain layer or a hook, never in components.
- **Branded types** for IDs and value objects — structural typing makes bare `string`s interchangeable; branded types fix that. Value objects enforce invariants at the boundary via factory functions that validate and throw typed `DomainError`s.
- **Hooks as ViewModels.** Dependencies flow inward: components → hooks → domain ports (repository interfaces); neither component nor hook knows a concrete API implementation. Custom hooks own state, call repository methods, and expose state + actions; the composition root (`App.tsx`/provider) wires concretes. Features must not import from each other.
- **Repository pattern for data.** Ports (interfaces) in `domain/ports/`; adapters in `data/repositories/` using `fetch`; a shared `ApiClient` in `data/api/` centralises base URL, auth headers, and error mapping. DTOs are separate types from domain entities — the repository maps between them and maps API/HTTP errors to typed `DomainError`s at the boundary.
- **CSS Modules + design tokens.** One co-located `.module.css` per component. All visual values reference `var(--tc-*)` tokens from `tokens.css` (this is what makes theming work). Mobile-first `@media` breakpoints (tablet 640px, desktop 1024px). CSS Module classes camelCase.
- **Strict TypeScript (non-negotiable):** `strict`, `noUncheckedIndexedAccess`, `noUnusedLocals`, `noUnusedParameters`, `noFallthroughCasesInSwitch`, `forceConsistentCasingInFileNames` all on.

## Test-double conventions (always applies)

- **Vitest + React Testing Library.** Custom hooks are the primary test target (they hold the orchestration logic, like Handlers/ViewModels); domain entities with business rules get direct unit tests; components are tested via integration render with real hooks. Red-Green-Refactor — write the test first.
- **Hand-written spies and fakes only** — consistent with the monorepo no-mocking-frameworks policy. **No `vi.fn()` / `vi.mock()` for repository dependencies** — write explicit spy classes that implement the port interface (e.g. `SpyPlanningApplicationRepository`, capturing `*Calls` arrays + `*Result` fields).
- **Fixtures are factory functions** returning domain entities with sensible defaults, overridable via spread (`overrides?: Partial<T>`) — the TypeScript equivalent of the Builder pattern (.NET) and static extensions (iOS).

## Forbidden

- `any` — use `unknown` and narrow with type guards (`any` silently disables TypeScript's safety).
- Importing React, browser APIs (`fetch`/`localStorage`/`window`), or any npm package inside `domain/` (only `import type` of shared API-contract types).
- `fetch` calls, complex state transitions, or business logic inside components — and components/hooks never call `fetch` directly (go through a repository).
- Cross-feature imports — features must not import from each other (share via `domain/` or `components/`).
- Class components — function components with hooks only.
- Default exports — named exports only.
- `<div onClick>` for clickable elements (breaks keyboard nav + screen readers) — use semantic HTML.
- Array index as a list `key` unless the list is static and never reordered.
- Inline `style={}` props — CSS Modules only.
- Hard-coded colors, spacing, font sizes, or border radii — always reference `var(--tc-*)` tokens.
- CSS-in-JS — no styled-components, Emotion, or Tailwind.
- `vi.fn()` / `vi.mock()` for repository dependencies — write explicit spy classes.
- Barrel files (`index.ts`) re-exporting everything (break tree-shaking, risk circular deps).
- `useEffect` for derived state — compute during render (`useMemo` only if genuinely expensive).
- Prop drilling beyond 2 levels — use context or restructure the tree.
- Mutating state directly — use spread / `structuredClone` for immutable updates.

## References (load on demand)

- `references/architecture-and-domain.md` — read when scaffolding the directory layout or a feature module, shaping domain entities/value-objects (branded types), or reasoning about Clean Architecture layering, hooks-as-ViewModel, and the cross-platform mapping table.
- `references/components-and-styling.md` — read when building or reviewing a component (structure, semantic HTML, accessibility, list keys) or writing CSS Modules, responsive layouts, or design-token styles.
- `references/data-access.md` — read when writing a repository port/adapter, an `ApiClient`, or DTO / error mapping between the API and domain.
- `references/testing.md` — read when writing any Vitest test, a hand-written spy, or a factory fixture (full examples: repository port, spy, fixture, hook test).
- `references/workflow-and-naming.md` — read when running dev/build/type-check/test/lint commands, configuring TypeScript strictness or ESLint, or checking naming conventions and best practices.
