# Escalation Protocol Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a two-tier decision escalation system so workers can escalate ambiguous decisions to the human via the Town Crier relay.

**Architecture:** New escalation-protocol skill defines the worker-side protocol (when/how to escalate, message format, stop-and-wait). Worker agents get a mandatory directive to invoke the skill. Team-lead skill gains background dispatch, an interleaved event loop, and relay rules.

**Tech Stack:** Claude Code skills (`.claude/skills/`), agent definitions (`.claude/agents/`), markdown

**Spec:** `docs/superpowers/specs/2026-03-20-escalation-protocol-design.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `.claude/skills/escalation-protocol/SKILL.md` | Create | Full escalation protocol — triggers, message format, stop-and-wait, re-escalation, mindset |
| `.claude/agents/dotnet-tdd-worker.md` | Modify | Add mandatory skill invoke after Inputs, remove old stub from Rules |
| `.claude/agents/ios-tdd-worker.md` | Modify | Same |
| `.claude/agents/react-tdd-worker.md` | Modify | Same |
| `.claude/agents/pulumi-infra-worker.md` | Modify | Same |
| `.claude/agents/github-actions-worker.md` | Modify | Same |
| `.claude/skills/team-lead/SKILL.md` | Modify | Background dispatch, interleaved event loop, relay rules, add react-tdd-worker |

---

### Task 1: Create the Escalation Protocol Skill

**Files:**
- Create: `.claude/skills/escalation-protocol/SKILL.md`

- [ ] **Step 1: Create the skill file**

```markdown
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
```

- [ ] **Step 2: Verify the file was created**

Run: `ls -la .claude/skills/escalation-protocol/SKILL.md`
Expected: File exists

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/escalation-protocol/SKILL.md
git commit -m "Add escalation protocol skill for worker decision escalation

Defines the two-tier escalation system: triggers, message format,
stop-and-wait behavior, re-escalation, and mindset guidance.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Update All Five Worker Agents

**Files:**
- Modify: `.claude/agents/dotnet-tdd-worker.md:17-18` (insert after Inputs, before Workflow)
- Modify: `.claude/agents/dotnet-tdd-worker.md:134` (remove old stub rule)
- Modify: `.claude/agents/ios-tdd-worker.md:17-18` (insert after Inputs, before Workflow)
- Modify: `.claude/agents/ios-tdd-worker.md:141` (remove old stub rule)
- Modify: `.claude/agents/react-tdd-worker.md:17-18` (insert after Inputs, before Workflow)
- Modify: `.claude/agents/react-tdd-worker.md:144` (remove old stub rule)
- Modify: `.claude/agents/pulumi-infra-worker.md:17-18` (insert after Inputs, before Workflow)
- Modify: `.claude/agents/pulumi-infra-worker.md:166` (remove old stub rule)
- Modify: `.claude/agents/github-actions-worker.md:17-18` (insert after Inputs, before Workflow)
- Modify: `.claude/agents/github-actions-worker.md:199` (remove old stub rule)

Each worker gets the same two edits:

- [ ] **Step 1: Add mandatory escalation section to dotnet-tdd-worker.md**

Insert after the Inputs section (after line 17 `Work in place.`) and before `## Workflow`:

```markdown

## Escalation Protocol (Mandatory)

Before starting any work, invoke the `/escalation-protocol` skill. This is not optional. The skill defines how and when to escalate decisions to the Town Crier. You must understand the escalation protocol before writing a single line of code.
```

- [ ] **Step 2: Remove old stub from dotnet-tdd-worker.md**

Delete this line from the Rules section:
```
- **Keep the team lead informed** — if you hit a blocker, report it clearly rather than guessing.
```

- [ ] **Step 3: Add mandatory escalation section to ios-tdd-worker.md**

Same insertion as Step 1, after the Inputs section and before `## Workflow`.

- [ ] **Step 4: Remove old stub from ios-tdd-worker.md**

Same deletion as Step 2.

- [ ] **Step 5: Add mandatory escalation section to react-tdd-worker.md**

Same insertion as Step 1, after the Inputs section and before `## Workflow`.

- [ ] **Step 6: Remove old stub from react-tdd-worker.md**

Same deletion as Step 2.

- [ ] **Step 7: Add mandatory escalation section to pulumi-infra-worker.md**

Same insertion as Step 1, after the Inputs section and before `## Tech Stack`.

- [ ] **Step 8: Remove old stub from pulumi-infra-worker.md**

Same deletion as Step 2.

- [ ] **Step 9: Add mandatory escalation section to github-actions-worker.md**

Same insertion as Step 1, after the Inputs section and before `## Tech Stack Context`.

- [ ] **Step 10: Remove old stub from github-actions-worker.md**

Same deletion as Step 2.

- [ ] **Step 11: Verify all five workers have the new section**

Run: `grep -l "Escalation Protocol (Mandatory)" .claude/agents/*.md`
Expected: All five worker files listed

Run: `grep -l "Keep the team lead informed" .claude/agents/*.md`
Expected: No files listed (old stub removed from all)

- [ ] **Step 12: Commit**

```bash
git add .claude/agents/dotnet-tdd-worker.md .claude/agents/ios-tdd-worker.md .claude/agents/react-tdd-worker.md .claude/agents/pulumi-infra-worker.md .claude/agents/github-actions-worker.md
git commit -m "Add mandatory escalation protocol to all worker agents

Each worker must invoke /escalation-protocol before starting work.
Removes the old 'keep the team lead informed' stub from all five
worker definitions.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Update Team-Lead Skill — Add react-tdd-worker and Fix Classification

**Files:**
- Modify: `.claude/skills/team-lead/SKILL.md:74` (agent type list)
- Modify: `.claude/skills/team-lead/SKILL.md:128-139` (classification heuristics)

- [ ] **Step 1: Add react-tdd-worker to the agent type list**

In the Agent Teams Protocol section, the `subagent_type` line (line 74) currently lists four types:

```
- `subagent_type`: the custom agent name (`ios-tdd-worker`, `dotnet-tdd-worker`, `pulumi-infra-worker`, or `github-actions-worker`)
```

Replace with:

```
- `subagent_type`: the custom agent name (`ios-tdd-worker`, `dotnet-tdd-worker`, `react-tdd-worker`, `pulumi-infra-worker`, or `github-actions-worker`)
```

- [ ] **Step 2: Verify react-tdd-worker appears in Phase 1 classification**

Read the classification section (lines 128-139). Confirm that `react-tdd-worker` is already listed in the Phase 1 heuristics (line 130: `**React/Web** → assign to react-tdd-worker`). If not, add it.

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/team-lead/SKILL.md
git commit -m "Add react-tdd-worker to team-lead agent type list

Pre-existing fix: the agent type list only mentioned four worker
types, missing react-tdd-worker.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Update Team-Lead Skill — Background Dispatch

**Files:**
- Modify: `.claude/skills/team-lead/SKILL.md:146-154` (Phase 2 dispatch template)

- [ ] **Step 1: Add run_in_background to the dispatch template**

The current Phase 2 Agent call template is:

```
Agent:
  subagent_type: "ios-tdd-worker" | "dotnet-tdd-worker" | "react-tdd-worker" | "pulumi-infra-worker" | "github-actions-worker"
  name: "aldric" — next unused peasant name from the roster
  team_name: "town-crier-guild"
  isolation: "worktree"
  model: "opus"
  mode: "bypassPermissions"
  prompt: "Work on bead `<bead-id>`."
```

Replace with:

```
Agent:
  subagent_type: "ios-tdd-worker" | "dotnet-tdd-worker" | "react-tdd-worker" | "pulumi-infra-worker" | "github-actions-worker"
  name: "aldric" — next unused peasant name from the roster
  team_name: "town-crier-guild"
  isolation: "worktree"
  model: "opus"
  mode: "bypassPermissions"
  run_in_background: true
  prompt: "Work on bead `<bead-id>`."
```

- [ ] **Step 2: Update the parallel dispatch note**

The note after the template (line 156) currently says:

```
**Parallel dispatch:** If multiple ready beads target different parts of the codebase (e.g., one iOS bead and one .NET bead), spawn all workers in a **single message** with multiple Agent tool calls. This runs them concurrently — each in its own isolated worktree. If two beads could touch overlapping files, dispatch them sequentially instead.
```

Replace with:

```
**Parallel dispatch:** Spawn all ready workers in a **single message** with multiple Agent tool calls, each with `run_in_background: true`. This runs them concurrently in isolated worktrees while keeping you free to relay decisions. If two beads could touch overlapping files, dispatch them sequentially instead. You are automatically notified when each background agent completes — do not poll.
```

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/team-lead/SKILL.md
git commit -m "Add run_in_background to team-lead dispatch template

Workers now run in background so Town Crier stays available to
relay decision escalations from workers to the human.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Update Team-Lead Skill — Interleaved Event Loop

**Files:**
- Modify: `.claude/skills/team-lead/SKILL.md:159-269` (Phases 3-6 replaced with event loop)

This is the largest change. Phases 3 (Validate), 4 (Merge Queue), 5 (Close the Bead), and 6 (Loop Until No Beads Remain) are replaced with a single "React Loop" phase. Phase 1 (Setup) and Phase 2 (Dispatch) remain, but the text after Phase 2's dispatch section is restructured.

- [ ] **Step 1: Replace Phases 3-6 with the React Loop**

After the Phase 2 dispatch section (ending with the parallel dispatch note), replace everything from `### Phase 3: Validate` through the end of `### Phase 6: Loop Until No Beads Remain` with:

```markdown
### Phase 3: React Loop

After dispatching workers, you enter the react loop. There is no polling — Claude Code delivers teammate messages and background completion notifications to you automatically. Handle each event as it arrives:

#### Event: DECISION NEEDED from a worker

A worker has sent a `SendMessage` containing `DECISION NEEDED [{bead-id}]`. This means the worker is **stopped and waiting** for an answer.

1. Collect all pending `DECISION NEEDED` messages received so far.
2. Surface them to the human in a **single** `AskUserQuestion` call. For each decision, include:
   - The worker's name
   - The bead ID
   - The worker's full message (verbatim — do not summarize, interpret, or filter)
3. When the human responds, relay each answer back to the corresponding worker:
   ```
   SendMessage(to: "{worker_name}"):
   DECISION [{bead-id}]

   {human's answer, verbatim}
   ```
4. The worker resumes upon receiving the response.

**You are a transparent pipe.** You never answer a decision yourself. Even if the question seems trivial or the answer seems obvious — relay it. The human decides what is trivial, not you.

#### Event: Worker completes (background agent notification)

A worker has finished and the Agent tool has returned its result including the worktree branch name.

1. **Validate** — run `bd show <bead-id>` and verify the notes contain:
   - A "TDD Evidence" (or "Infrastructure Evidence" or "Pipeline Evidence") section
   - Final test/build output showing success
   - At least one Red-Green-Refactor cycle documented (for TDD workers)

2. **Check commits** exist on the worktree branch:
   ```bash
   git log main..<branch-name> --oneline
   ```

3. **If validation passes** — add the branch to the merge queue.

4. **If validation fails** — spawn a **new** remediation worker (same type, next peasant name, `run_in_background: true`) with guidance to complete the evidence. Pass the existing worktree branch.

5. **Process the merge queue** when ready. Merge branches one at a time:
   ```bash
   git merge <branch-name> --no-edit
   ```

   - If clean: `git branch -d <branch-name>` and `git worktree prune`
   - If conflicts: `git merge --abort`, park the branch. After all clean merges, spawn a conflict resolver agent (same as current Phase 4 logic — `subagent_type: "general-purpose"`, `isolation: "worktree"`, `run_in_background: true`).

6. **Close the bead:**
   ```bash
   bd close <bead-id>
   ```

7. **Check for new work:**
   ```bash
   bd ready
   ```
   If new beads are ready, dispatch fresh workers (Phase 2) and continue the react loop.

#### Termination

The react loop ends when:
- All dispatched workers have completed
- All branches are merged (including conflict resolutions)
- All beads are closed
- `bd ready` returns zero beads

When done, sync beads:
```bash
bd dolt push
```

Do **not** `git push` unless the user explicitly asks.
```

- [ ] **Step 2: Verify the phase structure**

Read the updated file and verify:
- Phase 1 (Setup) is unchanged
- Phase 2 (Dispatch) includes `run_in_background: true`
- Phase 3 (React Loop) replaces the old Phases 3-6
- No orphaned phase references remain

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/team-lead/SKILL.md
git commit -m "Replace linear phases with interleaved react loop

Phases 3-6 are replaced by a single event loop that handles
decision relays, worker completions, merges, and new dispatches
as events arrive.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: Update Team-Lead Skill — Add Relay Rules

**Files:**
- Modify: `.claude/skills/team-lead/SKILL.md` (Rules section)

- [ ] **Step 1: Add relay rules to the Rules section**

Add the following rules to the existing Rules section (after the current list of rules):

```markdown
- **Never answer a decision yourself.** You are a relay, not a decision-maker. When a worker sends `DECISION NEEDED`, surface it to the human via `AskUserQuestion` and relay the human's answer back.
- **Always include bead ID and worker name** when surfacing decisions to the human.
- **Relay the human's answer verbatim** — do not interpret, summarize, or filter.
- **Even trivial questions get relayed** — the human decides what is trivial, not you.
- **Batch pending decisions** — if multiple `DECISION NEEDED` messages are waiting when you process them, combine them into a single `AskUserQuestion`.
```

- [ ] **Step 2: Verify the rules section**

Read the Rules section and confirm all five new rules are present alongside the existing rules.

- [ ] **Step 3: Commit**

```bash
git add .claude/skills/team-lead/SKILL.md
git commit -m "Add decision relay rules to team-lead skill

Town Crier must relay all worker decisions to the human verbatim,
batch pending decisions, and never answer decisions itself.

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: Final Verification

- [ ] **Step 1: Verify escalation-protocol skill exists and has correct frontmatter**

Run: `head -5 .claude/skills/escalation-protocol/SKILL.md`
Expected: frontmatter with `name: escalation-protocol`

- [ ] **Step 2: Verify all workers reference escalation-protocol**

Run: `grep -c "escalation-protocol" .claude/agents/*.md`
Expected: Each of the five files shows count of 1

- [ ] **Step 3: Verify no workers have the old stub**

Run: `grep -c "Keep the team lead informed" .claude/agents/*.md`
Expected: Each file shows count of 0

- [ ] **Step 4: Verify team-lead has run_in_background**

Run: `grep "run_in_background" .claude/skills/team-lead/SKILL.md`
Expected: At least one match

- [ ] **Step 5: Verify team-lead has react-tdd-worker**

Run: `grep "react-tdd-worker" .claude/skills/team-lead/SKILL.md`
Expected: At least one match

- [ ] **Step 6: Verify team-lead has relay rules**

Run: `grep "Never answer a decision yourself" .claude/skills/team-lead/SKILL.md`
Expected: One match

- [ ] **Step 7: Commit any remaining changes (if needed)**

If any verification steps revealed issues that were fixed, commit the fixes.
