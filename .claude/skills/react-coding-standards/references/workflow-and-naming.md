# Workflow, TypeScript Config & Naming (reference)

Read when running dev/build/type-check/test/lint commands, configuring TypeScript strictness, bootstrapping ESLint, or checking naming conventions and best practices. The core (`SKILL.md`) carries the strict-TS settings and the forbidden list; this file is the operational detail.

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
