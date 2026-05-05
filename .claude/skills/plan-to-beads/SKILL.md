---
name: plan-to-beads
description: "MUST use this skill whenever creating multiple beads from a plan, design doc, ADR, roadmap, or feature description. Raises the design as a GitHub issue (the source of truth), then creates lightweight beads that link to it. Trigger on: 'break down', 'create beads for', 'turn this into beads/tasks', 'set up the work for', 'populate the backlog', or any request to decompose a document into work items."
---

# Plan to Beads

Turn a plan into a GitHub issue (the design home) and lightweight beads that link to it.

## Philosophy

Beads track *what to do* and *dependencies*. The `how` and `why` live in **the GitHub issue body** — never in a committed spec file. Spec files in the repo rot faster than the code and mislead future readers; GitHub issues close when the work ships and stay searchable forever. ([Yegge, Issue #976](https://github.com/gastownhall/beads/issues/976))

**Hard rule: never create `docs/specs/*.md` or any committed spec/plan markdown.** If a worker needs context, the GitHub issue is where it lives.

## Workflow

### 1. Read the source material

The user will provide a plan, ADR, inline text, or file path. Read it carefully. If it references other documents (ADRs, product overview), read those too for context.

### 2. Raise the design as a GitHub issue

Use the `file-issue` skill (or `gh issue create` directly) to capture the full design context in the issue body. The issue is the source of truth for *how* and *why*. Structure it for the agent who'll implement the work:

```markdown
## Context
Why this work exists and what problem it solves.

## Design
How it should be built — architecture, patterns, constraints, key decisions.
Reference ADRs where applicable (e.g., "See ADR-0006 for polling model").

## Scope
What's in and out.

## Phases / Steps
Each phase is a sub-section that beads will link to via `#phase-1`-style anchors.

### Phase 1: <name>
What to build, key constraints, what done looks like.

### Phase 2: <name>
...

## Acceptance criteria
Testable checkboxes.
```

Capture the issue number/URL — every bead will reference it.

### 3. Design the bead hierarchy

Flat two-level: **epics** contain **tasks**. Each task = smallest scope producing a testable, meaningful outcome.

#### Consolidation pass

Merge plan items into a single bead when:
- Same files touched
- Thin slices of one concept
- Meaningless in isolation
- Shared setup dominates (80%+)

Do NOT merge when:
- Stories span different tech areas (different workers)
- Different dependency chains
- Combined scope too large (~3+ test cases = consider splitting)

### 4. Map dependencies

Data flow, API contracts, infrastructure, shared domain models. Wire with `bd dep add <issue> <depends-on>`.

### 5. Create lightweight beads

Present the hierarchy to the user for confirmation, then create:

```bash
bd create --title="<concise title>" \
  --description="GH: https://github.com/<org>/<repo>/issues/<n>#phase-1" \
  --type=task --priority=2 --parent=<epic-id> \
  --labels=api
```

Bead descriptions are **one line** — the GitHub issue reference (with anchor when there are multiple phases). The worker runs `gh issue view <n>` for full context.

Use `--priority`: P1 (foundational, blocks everything), P2 (default), P3 (nice-to-have).

Use `--labels` for tech area routing: `api`, `ios`, `web`, `infra`, `data`.

### 6. Verify

`bd list --status=open` — show what was created. Call out the dependency chain and the linked GitHub issue.

## Example

Given a plan for "PlanIt Polling Service" with 4 items:

1. Raise GitHub issue: `feat: PlanIt polling service` with the full design (Context, Design, Scope, Phases). Suppose it gets number `#234`.
2. Create beads:

```
Epic: "PlanIt polling service" (type=epic, P2)
  desc: "GH: https://github.com/<org>/town-crier/issues/234"
  |- Task: "PlanIt API client with response mapping and backoff" (P1, labels: api)
  |   desc: "GH: https://github.com/<org>/town-crier/issues/234#phase-1-api-client"
  |- Task: "Cosmos DB application upsert, idempotent on PlanIt name" (P1, labels: api,data)
  |   desc: "GH: https://github.com/<org>/town-crier/issues/234#phase-2-persistence"
  +- Task: "Background polling loop with configurable interval" (P2, labels: api)
      desc: "GH: https://github.com/<org>/town-crier/issues/234#phase-3-polling-loop"
      depends on: API client + persistence
```

Three lightweight beads, one GitHub issue with all the context.

## Constraints

- **Never** create a spec file in the repo (`docs/specs/*.md` or anywhere else).
- Always raise the GitHub issue before creating beads.
- Always get user confirmation before creating beads.
- Priority is 0-4 numeric, not "high"/"medium"/"low".
- If the plan references ADRs, read them for context on constraints.
