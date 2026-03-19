---
name: plan-to-beads
description: "MUST use this skill whenever creating multiple beads from a plan, spec, design doc, ADR, roadmap, or feature description. Converts plans into epic-and-task bead hierarchies with dependencies, acceptance criteria, and tech-area labels. Trigger on any of: 'break down' a feature/phase/plan into beads or tasks; 'create beads for' a feature or phase; 'turn this into beads/tasks'; 'set up the work for'; 'populate the backlog'; decomposing any document into actionable work items; or when the user references a plan/spec/roadmap and wants work items created from it. Also trigger when the user pastes an inline spec or feature description and asks to make it actionable or to create beads. Do NOT trigger for: creating a single bead (bd create), checking bead status (bd list/ready), closing beads, implementing code, or reviewing PRs."
---

# Plan to Beads

Turn a feature plan into a structured hierarchy of beads that AI agents can pick up and implement.

## Why this matters

A plan sitting in a markdown file is just prose. Agents need discrete, well-scoped work items with clear descriptions, acceptance criteria, and dependency ordering. This skill bridges that gap — it reads the plan, understands the structure, and creates beads that capture not just *what* to build but *why* it matters and *what done looks like*.

## Workflow

### 1. Identify the source material

The user will point you at one of:
- A specific phase or feature from `docs/feature-plan.md` (e.g., "break down Phase 1")
- A section of an ADR or design doc
- Inline text describing what they want built
- A file path to a spec or plan document

Read the source material carefully. If it references other documents (ADRs, product overview), read those too — they contain context that makes for better bead descriptions.

### 2. Design the hierarchy

Use a flat two-level hierarchy: **epics** contain **tasks**.

| Plan concept | Bead type | Example |
|-------------|-----------|---------|
| Phase or feature grouping | `epic` | "Phase 1 — Data Pipeline" |
| Small, testable unit of work | `task` | "Implement PlanIt API client with different_start filtering" |

Each task will be implemented by an AI agent, so scope them as the smallest testable unit of work — something an agent can complete in a single focused pass with a clear "done" signal. Think of a task as: one behaviour, one test, one commit.

If a plan feature (e.g., "1.1 PlanIt polling service") is small enough to be a single task, don't wrap it in an epic — just create it as a task. Only create an epic when there are genuinely multiple tasks to group.

### 3. Map dependencies

Dependencies tell agents what order to work in and what's blocked. Think about:
- **Data flow**: if task B reads data that task A writes, B depends on A
- **API contracts**: if the iOS app calls an API endpoint, the endpoint should exist first
- **Infrastructure**: database containers and deployment infra typically come before application code
- **Shared domain models**: value objects and entities used across tasks should be built first

Use `bd dep add <issue> <depends-on>` after creation. The dependent issue (the one that's blocked) comes first.

### 4. Write good bead descriptions

The description is a briefing for the agent who'll implement it. It should answer: *Why does this exist and what needs to happen?*

Include:
- **Context**: Why this piece matters in the bigger picture
- **Scope**: What's in and what's out
- **Key decisions**: Architectural constraints or patterns to follow (reference ADRs)
- **Technical pointers**: Relevant endpoints, data models, or existing code to build on

Keep descriptions concise — agents can read the codebase for details. Focus on intent and constraints they wouldn't discover by reading code alone.

### 5. Write acceptance criteria

Use the `--acceptance` flag to define what "done" looks like. Write concrete, verifiable statements that an agent can test against:

Good: "GET /api/applics/json called with different_start parameter; results upserted into Cosmos DB Applications container; upsert is idempotent on PlanIt name field; test covers duplicate handling"

Bad: "Polling works correctly"

### 6. Create the beads

Present the proposed hierarchy to the user first — show them what you plan to create (types, titles, parent-child relationships, dependencies) and get confirmation before running any `bd create` commands.

Once confirmed, create beads efficiently:
- Create epics first (they become parents)
- Then tasks as children of epics using `--parent`
- Add dependencies with `bd dep add` after all beads exist

Use `--priority` to reflect the plan's ordering:
- P1 for foundational work that blocks everything else
- P2 (default) for most work
- P3 for nice-to-haves or later-phase items

Use `--labels` to tag beads by tech area so they can be filtered and routed:
- `api` — .NET backend work
- `ios` — Swift/iOS app work
- `web` — React/TypeScript frontend work
- `infra` — Pulumi, CI/CD, deployment
- `data` — Cosmos DB schema, data modelling

A task can have multiple labels (e.g., `--labels api,data` for a repository implementation).

### 7. Verify and report

After creation, run `bd list --status=open` to show the user what was created. Call out the dependency chain so they can see the intended work order.

## Example

Given a plan entry like:

> **1.1 PlanIt polling service** — Background service polling `GET /api/applics/json?different_start={last_poll_iso}` on configurable interval (default 15 min), with rate limit handling and exponential backoff on 429s

You might create:

```
Epic: "PlanIt polling service" (type=epic, P2)
  ├─ Task: "Implement PlanIt API client with different_start filtering" (P1)
  ├─ Task: "Add rate limit detection and exponential backoff on 429s" (P2, depends on API client)
  └─ Task: "Create background polling loop with configurable interval" (P2, depends on API client)
```

Each task is one testable behaviour an agent can implement and verify independently.

## Important constraints

- Use `bd create` with `--title`, `--description`, `--type`, `--priority`, `--parent`, and `--acceptance` flags
- Never use `bd edit` — it opens an interactive editor that blocks agents
- Priority is 0-4 numeric (0=critical, 2=medium, 4=backlog), not "high"/"medium"/"low"
- Always show the proposed hierarchy and get user confirmation before creating beads
- If the plan references architectural decisions (ADRs), read them for context on constraints
