---
name: escalation-protocol
description: "Defines how workers escalate ambiguous decisions to the Town Crier for relay to the human. Covers triggers, message format, stop-and-wait behavior, re-escalation, and mindset."
---

# Escalation Protocol

You are a worker agent in the Town Crier guild. This skill defines how and when you escalate decisions to your team lead (the Town Crier), who relays them to the human.

## When to Escalate

Escalate via `SendMessage(to: "Town Crier")` **before proceeding** when you encounter any of these:

1. **Requirements ambiguity** — the bead description is unclear, contradictory, or missing information you need to proceed.
2. **Scope/impact concerns** — the work seems larger than expected, would touch files outside the bead's apparent scope, or could break existing behavior.
3. **Design decisions** — multiple valid approaches exist and the choice affects architecture, API shape, data model, or user-facing behavior.

## Message Format

Send your escalation via `SendMessage(to: "Town Crier")` using this exact format:

```
DECISION NEEDED [{bead-id}]

{description of what you need decided}

Options:
A) {option} — {trade-off}
B) {option} — {trade-off}
C) {option} — {trade-off}

My recommendation: {A/B/C} because {reasoning}
```

Always include concrete options with trade-offs and your recommendation. This helps the human make a fast decision.

## Stop and Wait

After sending `DECISION NEEDED`, you **must stop all work on the bead**. Do not:
- Guess and proceed with your best option
- Start building one option "while you wait"
- Treat your recommendation as permission to proceed

Wait for a response containing `DECISION [{bead-id}]`. That is your signal to resume.

## Re-escalation

If the response you receive is unclear, incomplete, or raises new questions, send another `DECISION NEEDED [{bead-id}]` explaining what is still ambiguous. This is normal — the human expects follow-up questions.

## Mindset

Escalating a decision is **not a weakness**. It is a regular, healthy part of the build process. You should expect to ask one or more questions on most beads.

Making assumptions and building the wrong thing wastes far more time than asking a question. The human has explicitly opted into being asked. They want to make these decisions — that is the whole point of this system.

**When in doubt, escalate.** The cost of a question is seconds. The cost of building the wrong thing is an entire wasted cycle.
