# Escalation Protocol Design

Date: 2026-03-20

## Overview

A two-tier decision escalation system for the Town Crier orchestration workflow. Workers escalate ambiguous decisions to the Town Crier (team lead), which relays them verbatim to the human via `AskUserQuestion`. The human answers, the Town Crier relays the answer back, and the worker resumes. The Town Crier never answers decisions itself — it is a transparent pipe.

## Motivation

Workers currently have a vague "keep the team lead informed" directive with no structured protocol. This leads to workers guessing when they hit ambiguity, building the wrong thing, and wasting cycles. The escalation protocol makes decision-seeking a first-class, structured, expected part of the build process.

## Architecture

### Flow

```
Worker                    Town Crier                 Human
  |                           |                        |
  |-- DECISION NEEDED ------->|                        |
  |   (SendMessage)           |-- AskUserQuestion ---->|
  |   [stops work]            |                        |
  |                           |<-- answer -------------|
  |<-- DECISION --------------|                        |
  |   (SendMessage)           |                        |
  |   [resumes work]          |                        |
```

### Key properties

- **Transparent relay**: Town Crier never interprets, filters, or answers decisions. It passes worker messages to the human and human answers back to workers verbatim.
- **Event-driven batching**: If multiple `DECISION NEEDED` messages are pending when the Town Crier processes them, they get batched into a single `AskUserQuestion`. No artificial delays — whatever is pending gets batched, single messages go immediately.
- **Interleaved main loop**: The Town Crier handles whatever event arrives — relay a decision, validate a completed worker, dispatch a new worker — in whatever order. No linear phases while workers are running.
- **Re-escalation**: Workers can send additional `DECISION NEEDED` messages if the human's answer is unclear or raises new questions.

## Component 1: Escalation Protocol Skill

**File**: `.claude/skills/escalation-protocol/SKILL.md`

A new skill that defines the full escalation protocol. Workers invoke it before starting work on any bead.

### When to escalate

Three triggers:

1. **Requirements ambiguity** — the bead description is unclear, contradictory, or missing information needed to proceed.
2. **Scope/impact concerns** — the work seems larger than expected, would touch files outside the bead's scope, or could break existing behavior.
3. **Design decisions** — multiple valid approaches exist and the choice affects architecture, API shape, data model, or user-facing behavior.

### Message format

Workers send via `SendMessage(to: "Town Crier")`:

```
DECISION NEEDED [{bead-id}]

{description of what you need decided}

Options:
A) {option} — {trade-off}
B) {option} — {trade-off}
C) {option} — {trade-off}

My recommendation: {A/B/C} because {reasoning}
```

### Stop and wait

After sending `DECISION NEEDED`, the worker stops all work on the bead. No guessing, no picking an option and proceeding. The worker waits for a response containing `DECISION [{bead-id}]` before resuming.

### Re-escalation

If the response is unclear or raises new questions, the worker sends another `DECISION NEEDED [{bead-id}]` explaining what is still ambiguous. This is normal and expected.

### Mindset

Escalating is a regular, healthy part of the build process. Workers should expect to ask one or more questions on most beads. Making assumptions and building the wrong thing wastes far more time than asking a question. The human wants to be asked — they have explicitly opted into this.

## Component 2: Worker Agent Changes

**Files**: All five worker agents in `.claude/agents/`:
- `dotnet-tdd-worker.md`
- `ios-tdd-worker.md`
- `react-tdd-worker.md`
- `pulumi-infra-worker.md`
- `github-actions-worker.md`

### Addition

A new mandatory section added near the top, after "Inputs" and before the workflow begins:

```markdown
## Escalation Protocol (Mandatory)

Before starting any work, invoke the `/escalation-protocol` skill. This is not optional.
The skill defines how and when to escalate decisions to the Town Crier. You must
understand the escalation protocol before writing a single line of code.
```

### Removal

Delete the existing rule from each worker:
> "Keep the team lead informed — if you hit a blocker, report it clearly rather than guessing."

The escalation skill replaces this entirely.

### No other changes

TDD workflow, evidence recording, commit process, and all other rules remain unchanged.

## Component 3: Team-Lead Skill Changes

**File**: `.claude/skills/team-lead/SKILL.md`

### Change 1: Background workers

In Phase 2 (Dispatch Workers), the Agent call template adds `run_in_background: true`. This keeps the Town Crier free to receive and relay messages while workers execute.

### Change 2: Interleaved event loop

After dispatching workers, the Town Crier handles events as they arrive instead of blocking:

- **`DECISION NEEDED` from a worker**: Collect all pending `DECISION NEEDED` messages. Surface them to the human in a single `AskUserQuestion`, identifying each by bead ID and worker name. When the human responds, relay each answer back to the corresponding worker via `SendMessage(to: "{worker_name}")` with the `DECISION [{bead-id}]` prefix.
- **Worker completion notification**: Proceed to validation and merge (existing Phase 3-5 logic).
- **New beads unblocked**: Dispatch fresh workers for newly ready beads.

### Change 3: New relay rules

Added to the Rules section:

- **Never answer a decision yourself.** You are a relay, not a decision-maker.
- **Always include bead ID and worker name** when surfacing decisions to the human.
- **Relay the human's answer verbatim** — do not interpret, summarize, or filter.
- **Even trivial questions get relayed** — the human decides what's trivial, not you.
- **Batch pending decisions** — if multiple `DECISION NEEDED` messages are waiting, combine them into a single `AskUserQuestion`.

## Files Changed

| File | Action | Description |
|------|--------|-------------|
| `.claude/skills/escalation-protocol/SKILL.md` | Create | Full escalation protocol skill |
| `.claude/skills/team-lead/SKILL.md` | Modify | Background workers, interleaved loop, relay rules |
| `.claude/agents/dotnet-tdd-worker.md` | Modify | Add mandatory skill invoke, remove old stub |
| `.claude/agents/ios-tdd-worker.md` | Modify | Add mandatory skill invoke, remove old stub |
| `.claude/agents/react-tdd-worker.md` | Modify | Add mandatory skill invoke, remove old stub |
| `.claude/agents/pulumi-infra-worker.md` | Modify | Add mandatory skill invoke, remove old stub |
| `.claude/agents/github-actions-worker.md` | Modify | Add mandatory skill invoke, remove old stub |

## Out of Scope

- Inter-worker coordination (two workers needing the same file) — handled by existing sequential dispatch for overlapping beads.
- Timeout handling for unresponsive humans — workers wait indefinitely.
- Decision logging/history beyond what beads already capture.
