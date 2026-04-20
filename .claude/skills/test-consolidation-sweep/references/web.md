# Web test consolidation (Vitest + Stryker-JS)

## Where tests live

- `web/src/**/__tests__/*.test.tsx` — component / page tests (React Testing Library)
- `web/src/**/__tests__/*.test.ts` — hook / adapter / domain tests

Hooks (used as ViewModels in this codebase per `react-coding-standards`) and domain/adapter code are the main consolidation targets. Component tests are often legitimately granular — each user-observable outcome usually deserves its own test.

## Test framework idioms

- `describe` / `it` blocks, `expect(...)` chains
- `it.each([...])` for parameterisation
- Hand-written spies/fakes (no `vi.mock` of internal modules)
- `@testing-library/react` for component tests (`render`, `screen`, `userEvent`)

## What to scan for

1. **`it` blocks sharing a `beforeEach`** and each asserting one behaviour. Target: `it.each([...])` parameterisation when inputs vary, or one verbose `it` with multiple assertions when the Act is identical.

2. **Hook tests asserting one returned value per test.** If the same `renderHook` call yields multiple fields and each field has its own `it`, consolidate into one verbose `it` that asserts all fields.

3. **Adapter tests enumerating HTTP verbs one per `it`.** Parameterise with `it.each([['GET', ...], ['POST', ...], ...])`.

4. **Domain tests enumerating valid/invalid inputs per `it`.** Parameterise: `it.each([['', 'rejects empty'], ['   ', 'rejects whitespace'], ...])`.

## Mutation testing — Stryker-JS

If not installed:

```bash
cd web
npm install --save-dev @stryker-mutator/core @stryker-mutator/vitest-runner
npx stryker init
```

Baseline run:

```bash
cd web
npx stryker run --mutate "<sut-files-glob>"
```

Workflow in the bead: record baseline, consolidate, re-run, require **≥** baseline. A drop is a failure.

## Naming target

Bad:

```ts
it('returns isLoading true while fetching');
it('returns empty items while fetching');
it('returns null error while fetching');
```

Good:

```ts
// Consolidates three separate `it`s asserting one returned field each.
// Rationale: the "loading" state is one observable shape; regressions should
// fail with the full picture, not one field at a time.
it('when fetching starts, the hook reports isLoading=true, items=[] and error=null', () => {
  const { result } = renderHook(() => useItems({ fetch: pendingFetch }));
  act(() => { result.current.start(); });
  expect(result.current.isLoading).toBe(true);
  expect(result.current.items).toEqual([]);
  expect(result.current.error).toBeNull();
});
```

## Special caution — component tests

Don't merge component tests where the user-facing outcome differs. A test "shows the error banner when the request fails" and "shows the spinner while the request is loading" are *different* observable states; they must stay separate. They are also the documentation of the component's observable behaviour.

The consolidation wins in `/web` live in **hooks and adapters**, not in components. A sweep that mostly flags component tests has likely misidentified the target — re-scan with hooks and adapters prioritised.

## Hook / feature floor

Every hook used as a ViewModel must retain at least one `it` whose description begins with `when <verb>ing` and covers the happy path — so the hook's behavioural contract is discoverable by grep.
