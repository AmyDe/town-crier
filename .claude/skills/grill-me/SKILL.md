---
name: grill-me
description: Interview the user relentlessly about a plan or design until reaching shared understanding, then publish a PRD as a GitHub issue with user stories arranged as strict vertical slices — the first story is the tracer bullet. Use when the user wants to stress-test a plan, get grilled on their design, or mentions "grill me".
---

## Phase 1 — Grill

Interview the user relentlessly about every aspect of this plan until you reach shared understanding. Walk down each branch of the design tree, resolving dependencies between decisions one-by-one. For each question, provide your recommended answer.

Ask **one question at a time**. Wait for the answer before asking the next.

If a question can be answered by exploring the codebase, explore the codebase instead of asking.

When the user uses a fuzzy or overloaded term, sharpen it before continuing — pin down a single canonical name and stick with it.

Stop grilling when there are no more open branches in the decision tree.

## Phase 2 — Synthesize the PRD

Once grilling is done, draft a PRD using the template below. Use the project's existing domain vocabulary throughout, and respect any ADRs in the area being touched.

### PRD template

```markdown
## Problem Statement
The problem the user is facing, from the user's perspective.

## Solution
The solution to the problem, from the user's perspective.

## User Stories (vertical slices)
A numbered list of user stories, ordered as strict **vertical slices**. Each slice cuts end-to-end through every layer it touches (schema, API, UI, tests) and is independently demoable.

Story 1 **must** be the **tracer bullet**: the thinnest end-to-end path that proves the architecture and integrations work. It should do almost nothing functionally — just light up every layer once.

Each story follows the format:
1. As a <actor>, I want <feature>, so that <benefit>

Prefer many thin slices over few thick ones.

## Implementation Decisions
Decisions made during grilling — modules, interfaces, schema changes, API contracts, architectural choices. Behavioural, not procedural. No file paths or line numbers (they go stale).

## Testing Decisions
What good tests look like for this feature, which modules will be tested, prior art for similar tests in the codebase.

## Out of Scope
What this PRD explicitly does NOT cover, to prevent gold-plating.

## Further Notes
Anything else worth recording.
```

### Vertical-slice rules

- Each slice delivers a narrow but **complete** path through every layer it touches.
- A completed slice is demoable or verifiable on its own.
- **Story 1 is the tracer bullet** — the simplest end-to-end story that proves the integration shape (e.g. one button → one API call → one row in the database → one passing test). It exists to de-risk, not to deliver value.
- Subsequent stories thicken the bullet: more inputs, more edge cases, more UI surface, more validation.
- Never reorganise into "build the backend first, then the frontend" — that breaks the slice.

## Phase 3 — Publish the PRD as a GitHub issue

Confirm the issue title with the user, then publish via `gh`:

```bash
gh issue create \
  --title "<short feature name>" \
  --body "$(cat <<'EOF'
<PRD body here>
EOF
)"
```

Print the issue URL back to the user when done.

Do **not** create beads or worktrees here — decomposition into trackable work is a separate step (`/plan-to-beads`), kept out of the grilling skill so each skill stays sharp.
