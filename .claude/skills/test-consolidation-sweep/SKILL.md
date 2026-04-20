---
name: test-consolidation-sweep
description: "Autonomous test-consolidation auditor — scans TUnit/.NET, Swift Testing/iOS, and Vitest/React test suites for over-granular tests that can be merged, parameterised, or folded into verbose behaviour-preserving tests WITHOUT losing coverage. Files one bead per consolidation opportunity with a mutation-testing gate to prove coverage holds. Designed for daily /loop. MUST use this skill whenever the user says 'test consolidation', 'reduce test count', 'consolidate tests', 'test sweep', 'too many tests', 'slim down tests', 'test audit', 'merge tests', 'parameterize tests', 'parameterise tests', or any variation of wanting to reduce the size of the test suite without losing coverage."
---

# Test Consolidation Sweep

Scan the Town Crier test suites (.NET, iOS, web), find tests that can be consolidated into fewer, verbose, coverage-preserving tests, and raise one lightweight bead per finding. Each bead encodes a mutation-testing gate so the worker implementing the consolidation can prove coverage hasn't degraded.

Runs daily via `/loop` — be idempotent.

## Why this skill exists

A codebase accumulates one tiny test per bead over time. Granular tests feel safe, but:

- They make intent harder to read — you scan 15 `Should_X_When_Y` tests instead of one verbose test that tells the story in its name and its asserts.
- They bloat the suite and slow CI.
- They couple tests to implementation shape rather than behaviour.
- Most importantly, many tests can still leave branches unmutated — mutation testing usually exposes this.

Consolidation is not deletion. It is compression: same assertions, same branches, fewer files, clearer names.

## Consolidation principles — apply to every finding

**Direction is always inward and downward, never upward.**

1. **Handler tests are the coverage layer.** All branch/edge-case coverage lives at the handler level — the handler is a black box, and the tests exercise its inputs and observe its outputs. Consolidation happens *within* a handler's test class: merge shared-setup tests, parameterise input-variant tests, fold small single-assert tests into fewer verbose multi-assertion tests.

2. **Flow/integration tests are the silver bullet.** One happy path through a flow to prove wiring works. If a flow test has accreted error-path assertions that already exist at handler level, **trim them from the flow test** — do not delete the flow test. Never move coverage *up* from handler → flow; that moves assertions to the wrong layer and dilutes the silver-bullet purpose.

3. **No cross-bounded-context consolidation in unit tests.** Merging unit tests across BCs loses the "what broke and where" signal. Integration/e2e tests may legitimately cross BCs when they exercise an integration seam.

4. **Floor per handler: enough tests for distinct behavioural branches.** Don't reduce a handler to zero tests. Don't smash unrelated behaviours into one test. Each distinct branch should have a named test surfacing it.

5. **Naming: verbose and behavioural.** Target `When_<situation>_it_<outcome1>_and_<outcome2>_and_<outcome3>`. Comments above the test documenting *why* consolidation happened (e.g. "these three behaviours are one observable state — regressions should fail with full context") are welcome and often helpful.

6. **Mutation-testing gate is mandatory.** Every bead must require the worker to run a mutation baseline on the SUT files *before* consolidation, then re-run *after*. The bead aborts if the mutation score drops. Line/branch coverage alone is too easy to game — only mutation survival proves the consolidation didn't silently lose coverage.

## Execution

```
Check existing beads → Scan all stacks → Validate findings → Create beads → Report
```

## Phase 1: Load existing beads

```bash
bd search "consolidate tests"
bd search "test consolidation"
bd list --status=open
bd list --status=in_progress
```

A finding is a duplicate if an existing open bead covers the **same test file(s) + same consolidation kind**. Don't re-raise closed work unless a new test has been added to the affected file(s) since close.

## Phase 2: Per-stack scan

Scan stacks in parallel via subagents where possible. Load the relevant reference file per stack — each reference tells you the local test framework, file conventions, assertion idioms, and mutation tool setup:

- **.NET** (`api/tests`) — see [references/dotnet.md](references/dotnet.md) — TUnit, Stryker.NET
- **iOS** (`mobile/ios/town-crier-tests`) — see [references/ios.md](references/ios.md) — Swift Testing, muter (with a textual-assertion fallback when muter can't run cleanly)
- **Web** (`web/src/**/__tests__`) — see [references/web.md](references/web.md) — Vitest, Stryker-JS

Within each stack the scan looks for four patterns (described in detail in the references):

- **Shared-setup merge** — multiple tests in one class/suite that build the same SUT and differ only in the Act input
- **Parameterisation** — tests whose bodies differ only in a scalar or enum input
- **Tiny single-assert tests** — tests on the same Act asserting one field each
- **Flow test over-reach** — integration/flow tests asserting details that belong at handler level

## Phase 3: Validate findings

For each candidate, confirm:

- The consolidation direction is inward/downward only (never upward).
- The tests are in the **same bounded context** at the unit level, or legitimately span BCs at integration/e2e level.
- The consolidated test name would meaningfully describe behaviour, not just enumerate steps.
- The SUT files that these tests cover are reachable by the relevant mutation tool (or, for iOS only, the assertion-preservation fallback applies — see references/ios.md).

## Phase 4: Create beads

One bead per consolidation opportunity. Keep beads lightweight — 4–6 lines describing the before state, the proposed after state, the SUT files under the mutation gate, and any rationale worth preserving as a comment in the consolidated test.

```bash
bd create \
  --title="Consolidate tests: <test class / area>" \
  --description="Files: <test file(s)>. Current: <N tests>. Proposed: <M tests>, pattern=<parameterise|merge-shared-setup|fold-asserts|flow-trim>. SUT under mutation gate: <paths>. Naming target: When_<situation>_it_<outcomes>. Rationale to preserve as comment: <if any>." \
  --type=task \
  --priority=3 \
  --acceptance="Mutation score on SUT files is >= baseline (no drop tolerated). No coverage assertions moved up from handler to flow. Consolidated test name follows When_<situation>_it_<outcomes> pattern. Handler floor respected (distinct behavioural branches still have named tests)."
```

Priority:

- **P3** — within-class mergers, parameterisation, flow-test trimming (low risk).
- **P2** — cross-class consolidation within a single bounded context (riskier; merits reviewer attention).

## Phase 5: Report

```
Test consolidation sweep: created N beads (X within-class mergers, Y parameterisation, Z flow-trims)
```

Or: `Test consolidation sweep: no new findings`

Or: `Test consolidation sweep: created 3 beads, skipped 5 (already tracked)`

## Idempotency

- Don't duplicate existing open beads for the same test file + consolidation kind.
- Don't re-raise closed work unless new tests have been added to the file(s) since close.
- If `git log --since="24 hours ago" -- api/tests mobile/ios/town-crier-tests web/src` is empty, exit quickly with "no test changes since last sweep".

## What NOT to flag

- **Tests with genuinely distinct names and assertions** describing distinct behaviours — many tests that are each telling a different part of the story are not duplication.
- **Unit tests across bounded contexts** — merging them loses BC clarity.
- **Integration tests as candidates for absorbing handler-level coverage** — keep the silver-bullet lean.
- **One-test handlers** — don't reduce to zero, and don't smash together unrelated handler tests.
- **Tests that are slow but distinct** — slowness is not duplication; propose splits for that, not merges.
- **Regression tests named after a bug ID or issue number** — intentionally narrow and load-bearing; keep them.
- **Component tests asserting distinct user-facing outcomes** (e.g. spinner vs error banner) — see `references/web.md`.

## When tempted to propose an upward merge

If several handler tests look like they could all be "covered" by one flow test, **don't** propose that merge. Propose *within-handler* consolidation instead. The flow test, if it exists, remains a silver bullet. If no flow test exists and one should, file a *separate* bead proposing it as **new coverage**, not as a consolidation of handler tests.
