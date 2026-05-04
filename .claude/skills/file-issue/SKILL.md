---
name: file-issue
description: "Research a bug or feature description, resolve all design decisions, then raise a deeply detailed GitHub issue ready for autonomous implementation. The output issue must be self-contained — downstream triage and autopilot sessions implement it without any further questions. TRIGGER on: 'file an issue', 'raise an issue', 'create a github issue', 'log a bug', 'file a feature request', or when the user describes something they want built or fixed."
---

# File Issue

You are a senior engineer who has just received a human-language description of a bug or feature. Your job is to transform it into a GitHub issue so detailed and so complete that an autonomous agent can implement it correctly with zero follow-up questions.

Downstream sessions (triage-inbox, autopilot) run without human oversight. Every implementation ambiguity you leave open will either block the worker or produce the wrong thing. **Ambiguity is a defect. Resolve it here.**

## Execution Flow

```
Parse request → Research (codebase + history + GitHub + infra) → Identify decisions → Resolve decisions from research → Ask user only what research can't answer → Draft issue → Create issue
```

---

## Phase 1: Parse the Request

Read the user's description and extract:

- **Type:** bug | feature | enhancement | chore
- **Area(s):** api | ios | web | infra | ci | data — infer from description keywords; multiple is fine
- **Core ask:** one sentence — what does the user actually want?
- **Signals for research:** terms, entity names, endpoint names, screen names, error messages to grep for

Do NOT ask clarifying questions yet. Research first.

---

## Phase 2: Research

Run all applicable probes in parallel. Read widely — a 60-second investment here prevents a wasted worker cycle.

### 2a. Codebase — find the blast radius

Grep for the terms you identified. For each significant hit, read the file around it.

```bash
# Find relevant files — search all code extensions
grep -r "<key term>" . \
  --include="*.cs" --include="*.swift" --include="*.tsx" --include="*.ts" \
  -l --exclude-dir=".git" --exclude-dir="node_modules" --exclude-dir=".build"

# Find specific patterns (endpoint names, type names, handler names)
grep -r "<term>" . \
  --include="*.cs" --include="*.swift" --include="*.tsx" -n \
  --exclude-dir=".git" --exclude-dir="node_modules" -m 5
```

Read the ADRs if the request touches a core architectural pattern:
```bash
ls docs/adr/ && grep -l "<relevant term>" docs/adr/*.md
```

Search prior GitHub issues for context on related work:
```bash
gh issue list --search "<relevant term> in:title,body" --state=all --limit=10
```

For API work — find the handler, command/query, repository, and domain model:
```bash
find api/src -name "*.cs" | xargs grep -l "<handler or entity name>" 2>/dev/null
```

For iOS work — find the ViewModel, UseCase, Repository:
```bash
find mobile/ios -name "*.swift" | xargs grep -l "<viewmodel or usecase name>" 2>/dev/null
```

For web work — find the feature slice, hook, component:
```bash
find web/src -name "*.tsx" -o -name "*.ts" | xargs grep -l "<component or hook name>" 2>/dev/null
```

### 2b. Git history — understand recent changes and context

```bash
# Recent changes to relevant files
git log --oneline -20 -- <relevant-file-path>

# Commits related to the feature/bug by keyword
git log --oneline --grep="<keyword>" --all -10

# Last 15 commits on main (orientation)
git log --oneline -15
```

Read commit messages for any that look related. If a fix was reverted or a feature was partially implemented, that context matters.

### 2c. GitHub — find existing work

```bash
# Open issues on same topic
gh issue list --search "<key term>" --state open --limit 10 --json number,title,labels,url

# Closed issues (to understand prior attempts)
gh issue list --search "<key term>" --state closed --limit 5 --json number,title,url

# Related open PRs
gh pr list --search "<key term>" --state open --limit 5 --json number,title,url

# Open beads on same topic
bd search "<key term>" 2>/dev/null | head -20
```

If an open GH issue already covers exactly this request, **do not file a duplicate**. Instead tell the user which existing issue covers it and ask if they want to add context to that one.

### 2d. Infrastructure (only if relevant to the request)

Only run these if the request involves deployment config, environment variables, Azure resources, costs, or scaling:

```bash
# Current container app config
az containerapp show -n ca-town-crier-api-prod -g rg-town-crier-shared --query "{replicas:properties.template.scale, image:properties.template.containers[0].image, envVars:properties.template.containers[0].env}" -o json 2>/dev/null

# Recent deployments
gh run list --workflow="CD Production" --status completed --limit 5 --json databaseId,conclusion,startedAt,headBranch -q '.[] | "\(.startedAt) \(.conclusion) \(.headBranch)"'
```

---

## Phase 3: Identify and Resolve All Decisions

After research, produce an internal list of every decision an implementer would need to make. For each one, either **resolve it from research** or flag it as **needs user input**.

### What to resolve autonomously (do not ask the user):

- **Which files to change** — you found them in the grep.
- **Which pattern to follow** — existing code shows the convention (TDD, handler style, CosmoS SDK usage, etc.).
- **Test strategy** — follow the existing test pattern for the area (TUnit for .NET, XCTest for iOS, Vitest for web).
- **Error handling approach** — follow the pattern already in adjacent code.
- **Naming conventions** — follow what the codebase already uses.
- **Implementation detail within a settled architecture** — e.g. "add a property to an existing domain entity" is not a design decision.

### What requires user input (ask these only if research can't answer):

- **User intent / business rule** — "what should happen when X?" where X is a policy the user owns.
- **Priority call** — two valid approaches with different trade-offs where the right choice depends on direction only the user knows.
- **Scope decision** — "should this also cover Y?" where Y is adjacent work the user may or may not want.
- **Data you can't verify** — e.g. an error message from a live system you can't reproduce locally.

If you have questions, collect them ALL now. You will ask in one batch in Phase 4.

---

## Phase 4: Ask Questions (only if needed)

If you have unresolvable questions, present them ALL at once:

> Before I file this issue I need to resolve a few things. Research covered most decisions but these need your input:
>
> 1. [Question] — [why you can't infer this from code] — My default if you don't answer: [what you'll assume]
> 2. [Question] — ...
>
> Reply with answers by number, or say "use your defaults" if happy with them.

**Rules for questions:**
- Maximum 4 questions. If you have more, make the decisions yourself and note them in the issue.
- Each question must include your default assumption — user can always just say "defaults are fine".
- Never ask for information that's in the codebase. Go find it.
- Never ask the user to explain the codebase to you.

If you have zero questions, skip Phase 4 entirely and proceed to Phase 5.

Wait for the user's response before continuing.

---

## Phase 5: Draft the Issue

Write a GitHub issue body that leaves nothing for the implementer to figure out. Use the template below.

The bar: a junior developer who has never seen this codebase should be able to implement it correctly from this issue alone — with the relevant files named, the approach decided, the edge cases handled, and the tests specified.

### Issue Title

Format: `<type>: <imperative sentence describing the change>`

Examples:
- `bug: digest emails sent with wrong timezone`
- `feat: add watchlist count to home screen badge`
- `chore: remove legacy PlanIt v1 polling path`

### Issue Body Template

```markdown
## Summary

<!-- One sentence. What changes. -->

## Motivation

<!-- Why this matters. User impact. What breaks or is missing today. -->
<!-- For bugs: include reproduction context if known. -->

## Current Behaviour

<!-- What happens today. Be specific. -->

## Desired Behaviour

<!-- What should happen after this ships. Be specific. -->

## Technical Context

<!-- Research findings that the implementer needs. Include: -->
<!-- - Relevant files with paths -->
<!-- - How adjacent code handles similar cases (cite file:line) -->
<!-- - Any ADR or spec that governs this area -->
<!-- - Known gotchas or constraints from your research -->

## Proposed Approach

<!-- The implementation plan. Be specific about the approach chosen. -->
<!-- If there were alternatives, note them here and explain why you rejected them. -->
<!-- This section is the implementer's roadmap — they should not need to invent anything. -->

## Pre-Resolved Design Decisions

<!-- Decisions the implementer does NOT need to make. State each one explicitly. -->
<!-- Format: Decision: [what was decided]. Rationale: [why]. -->
<!-- This section exists to prevent the worker from pausing to ask. -->

## Acceptance Criteria

<!-- Testable, specific checkboxes. -->
<!-- Each one should be verifiable from code or a test result alone. -->
- [ ] ...
- [ ] ...

## Test Strategy

<!-- What tests to write. Name the test class/file if you know it. -->
<!-- Specify the test framework (TUnit, XCTest, Vitest). -->
<!-- Name specific scenarios to cover: happy path, edge cases, error cases. -->

## Out of Scope

<!-- What NOT to change. Common mistakes to avoid. Boundary conditions. -->
<!-- This section prevents over-engineering and scope creep. -->

## Files to Change

<!-- Exhaustive list of files to create or modify. -->
<!-- One line per file: `path/to/file.ext` — what to do there -->

## References

<!-- Related issues, PRs, ADRs, specs, commits. -->
```

### Filling the template

- **Never leave a section empty** — if it's not applicable, delete the section.
- **Files to Change must be exhaustive** — if you found the files in Phase 2, list them. If the worker needs to create new files, say so and give the exact path following existing naming conventions.
- **Acceptance criteria must be automatable** — "tests pass" is not a criterion. "A new TUnit test `WhenDigestIsSent_TimezoneIsUserLocal` passes" is.
- **Pre-Resolved Design Decisions is critical** — this is where you deposit all the thinking you did so the worker doesn't redo it. Even obvious-to-you decisions should be stated explicitly.

---

## Phase 6: Set Labels

Determine the correct labels:

| Condition | Label |
|---|---|
| User reported something broken | `bug` |
| New capability | `enhancement` |
| Internal improvement (no user-visible change) | (none, just describe it) |

Do NOT add area labels — `triage-inbox` assigns those from the issue body when it creates the bead.

---

## Phase 7: Create the Issue

```bash
gh issue create \
  --title "<title>" \
  --body "$(cat <<'BODY'
<full body>
BODY
)" \
  --label "<label>"
```

After creating, print:
```
Filed: <issue URL>
Title: <title>
Triage-inbox will pick this up on its next loop tick.
```

---

## Worked Examples

### Bug example output (Pre-Resolved Decisions section)

> **Decision:** Use the `UserTimezone` property already stored on the `UserProfile` document in Cosmos — do not fetch it from Auth0. **Rationale:** `UserProfile` is the authoritative source (CLAUDE.md: "query Cosmos DB first"). Auth0 may be unavailable; Cosmos already has the value from onboarding.
>
> **Decision:** If `UserTimezone` is null or invalid, fall back to `Europe/London`. **Rationale:** Matches the app's target market and prevents a null-reference exception. Filed as a separate cleanup bead: #NNN.

### Feature example output (Acceptance Criteria section)

> - [ ] `GET /v1/me/watchlist` returns a `count` field equal to the number of active watches
> - [ ] `count` is 0 when the user has no watches (not omitted, not null)
> - [ ] A new TUnit test `WhenUserHasWatches_CountMatchesRepository` passes
> - [ ] A new TUnit test `WhenUserHasNoWatches_CountIsZero` passes
> - [ ] No change to existing response fields — this is purely additive

---

## Rules

- **Research before asking.** Never ask a question answerable from the codebase.
- **Batch all questions.** One round only, with defaults. Never iterate.
- **Resolve everything.** The issue is defective if an implementer would pause and wonder.
- **No duplicate issues.** Check GitHub and beads before filing.
- **Exhaustive file list.** Every file the worker will touch must be named.
- **Acceptance criteria are automatable.** If a human must manually verify it, it's too vague.
- **Short title, rich body.** Title is for scanning; body is for implementing.
