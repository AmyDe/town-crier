---
name: simplify-sweep
description: "Aggressive autonomous code simplification auditor — scans the entire codebase hunting for dead code, over-abstraction, duplication, and unnecessary complexity, then raises one lightweight bead per finding. Designed for daily `/loop`. MUST use this skill whenever the user says 'simplify', 'simplification sweep', 'find dead code', 'reduce complexity', 'code audit', 'refactoring opportunities', or any variation."
---

# Simplify Sweep

Scan the Town Crier codebase, find code that can be made simpler without changing behaviour, and raise one lightweight bead per finding. Runs daily via `/loop` — be idempotent.

## Execution

```
Check existing beads -> Scan all tech areas -> Validate findings -> Create beads -> Report
```

## Phase 1: Load Existing Beads

```bash
bd search "simplify"
bd list --status=open
bd list --status=in_progress
```

Build inventory of what's already raised. A finding is a duplicate if an existing open bead covers the same file + same kind of simplification. Don't re-raise closed work unless code regressed.

## Phase 2: Deep Codebase Scan

Scan everything using parallel subagents. Be aggressive — flag anything that could be simpler.

### What to hunt for

**API (`/api`):** value objects wrapping a single primitive with no behaviour, handlers mixing orchestration + business logic, duplicated Cosmos DB query patterns, unused interfaces with single implementations and no test doubles.

**iOS (`/mobile/ios`):** ViewModels with domain logic, single-conformance protocols with no spy, coordinators doing work simpler SwiftUI navigation handles, duplicated view modifiers, dead `@Published` properties.

**Web (`/web`):** components doing too many things, hooks wrapping a single `useState`, duplicated API call patterns, unused CSS Module classes, `any` types.

**Infra (`/infra`):** unnecessary Pulumi config (matching Azure defaults), duplicated resource patterns, stale outputs.

**CI/CD (`.github/workflows/`):** duplicated steps across workflows, dead jobs, over-complex conditionals.

### How to scan

1. Read directory trees with `Glob`
2. Read source files — focus on files >100 lines and high-import files
3. Use `Grep` for unused exports, commented-out code, dead code markers
4. Cross-reference: confirm "unused" is truly unreferenced

### What NOT to flag

- Architecture-mandated patterns (hexagonal ports, CQRS handlers, MVVM-C coordinators)
- Abstractions enabling testing where test doubles exist
- Genuine complexity, style issues, performance optimisations

## Phase 3: Validate Findings

For each finding: confirm it's real (grep for symbol), confirm it's simplifiable, check if already raised.

## Phase 4: Create Beads

```bash
bd create \
  --title="Simplify: <concise description>" \
  --description="<file>:<lines> — <what's there now>. Simplify to: <simpler version>. Tests: <test file> covers this / write tests first." \
  --type=task \
  --priority=3
```

Descriptions are **2-3 lines** — file path, current state, simpler state, test status. Workers read the code for details.

Priority: P3 (dead code, unused imports) or P2 (over-abstraction, duplication).

## Phase 5: Report

```
Simplify sweep: created N beads (X dead code, Y over-abstraction, Z duplication)
```

Or: `Simplify sweep: no new findings`

Or: `Simplify sweep: created 3 beads, skipped 5 (already tracked)`

## Idempotency

- Don't duplicate existing open beads for same file + finding
- Don't re-raise closed work unless code regressed
- Quick exit if `git log --since="24 hours ago" --oneline` is empty (lighter scan)
