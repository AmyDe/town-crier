---
name: plan-to-beads
description: "MUST use this skill whenever creating multiple beads from a plan, spec, design doc, ADR, roadmap, or feature description. Creates a spec file first, then lightweight beads referencing it. Trigger on: 'break down', 'create beads for', 'turn this into beads/tasks', 'set up the work for', 'populate the backlog', or any request to decompose a document into work items."
---

# Plan to Beads

Turn a plan into a spec file and lightweight beads that point to it.

## Philosophy

Beads track *what to do* and *dependencies*. Specs capture *how* and *why*. This separation keeps the issue tracker fast and scannable while preserving rich context in docs. ([Yegge, Issue #976](https://github.com/gastownhall/beads/issues/976))

## Workflow

### 1. Read the source material

The user will provide a plan, spec, ADR, inline text, or file path. Read it carefully. If it references other documents (ADRs, product overview), read those too for context.

### 2. Create the spec file

Write a spec to `docs/specs/<topic>.md` capturing the full design context. This is the source of truth for *how*. Write it for the agent who'll implement the work:

```markdown
# <Feature/Phase Name>

## Context
Why this work exists and what problem it solves.

## Design
How it should be built — architecture, patterns, constraints, key decisions.
Reference ADRs where applicable (e.g., "See ADR-0006 for polling model").

## Scope
What's in and out.

## Steps

### <Step 1 name>
What to build, key constraints, what done looks like.

### <Step 2 name>
...
```

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

Data flow, API contracts, infrastructure, shared domain models. Wire with `/beads:dep add <issue> <depends-on>`.

### 5. Create lightweight beads

Present the hierarchy to the user for confirmation, then create:

```bash
bd create --title="<concise title>" \
  --description="Spec: docs/specs/<topic>.md#<section>" \
  --type=task --priority=2 --parent=<epic-id> \
  --labels=api
```

Bead descriptions are **one line** — the spec reference. The worker reads the spec for full context.

Use `--priority`: P1 (foundational, blocks everything), P2 (default), P3 (nice-to-have).

Use `--labels` for tech area routing: `api`, `ios`, `web`, `infra`, `data`.

### 6. Verify

`/beads:list --status=open` — show what was created. Call out the dependency chain.

## Example

Given a plan for "PlanIt Polling Service" with 4 items:

1. Create spec: `docs/specs/planit-polling.md` with full design context
2. Create beads:

```
Epic: "PlanIt polling service" (type=epic, P2)
  |- Task: "PlanIt API client with response mapping and backoff" (P1, labels: api)
  |   desc: "Spec: docs/specs/planit-polling.md#api-client"
  |- Task: "Cosmos DB application upsert, idempotent on PlanIt name" (P1, labels: api,data)
  |   desc: "Spec: docs/specs/planit-polling.md#persistence"
  +- Task: "Background polling loop with configurable interval" (P2, labels: api)
      desc: "Spec: docs/specs/planit-polling.md#polling-loop"
      depends on: API client + persistence
```

Three lightweight beads, one spec file with all the context.

## Constraints

- Always create the spec file before creating beads
- Always get user confirmation before creating beads
- Priority is 0-4 numeric, not "high"/"medium"/"low"
- If the plan references ADRs, read them for context on constraints
