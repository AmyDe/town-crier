# Autopilot: Split Multi-Worker Beads

Date: 2026-04-22

## Context

The `autopilot` skill picks one ready bead per loop tick, classifies it to a single worker type (iOS, .NET, React, Pulumi, GitHub Actions, delete), and dispatches that worker in an isolated worktree. The classifier maps a bead to exactly one worker via a signals table.

When a bead legitimately spans multiple worker types — e.g. a feature that adds an API endpoint AND an iOS client AND a web client, or a migration that touches both `/api` handlers and `/infra` config — the current skill has no path that works. It either mis-classifies to the first-matching worker (who then can't finish because changes are gated to one path), or it marks the bead blocked as "unclassifiable." Either way, multi-area work silently stops the loop.

This is a real pattern: features commonly span API + one or more clients; migrations span code + infra + CI/CD.

## Decision

Extend autopilot with a **split step** that runs between classification and claim. When a bead requires more than one worker type, autopilot splits it into N child beads (one per worker type), marks the parent as superseded, adds dependencies where one child produces a contract the other consumes, and returns. The next loop tick picks up the unblocked child(ren) via the normal `/beads:ready` flow.

### Trigger

After `/beads:show`, autopilot reads the bead's title, description, acceptance, design notes, and any referenced spec file (e.g. `docs/specs/<topic>.md#phase-1`). It splits when that reading reveals work requiring **more than one worker type**.

**Split:**
- Feature spec lists `/api` endpoint + `/mobile/ios` UI + `/web` UI.
- Migration touches `/api` handlers and `/infra` config.

**Do NOT split:**
- Signals point to exactly one worker → normal flow.
- Multiple tech areas mentioned but work is clearly in one area (e.g. "fix iOS bug triggered by an earlier API change" — only iOS work to do) → normal flow.
- Ambiguous which files belong to which worker → mark parent blocked for human triage. Don't force a bad split.

### Split mechanics

For each worker type needed, create a child bead:

```bash
bd create \
  --title="<parent title> — <area>" \
  --description="<slice description, referencing parent>" \
  --type=<inherited from parent> \
  --priority=<inherited from parent> \
  --acceptance="<slice-scoped acceptance>" \
  --notes="Split from <parent-id> by autopilot. Worker: <worker-type>. Allowed path: <path>."
```

Title convention: `<original title> — API`, `<original title> — iOS`, `<original title> — web`, etc. Type, priority, and any spec-file references in the parent description carry into each child.

Link the parent:

```bash
bd supersede <parent-id> --with=<child-1-id>
# If bd supersede accepts only one --with, record the rest in notes:
bd update <parent-id> --append-notes="Also superseded by: <child-2-id>, <child-3-id>"
bd update <child-2-id> --append-notes="Split sibling of <child-1-id>. Parent: <parent-id>."
bd update <child-3-id> --append-notes="Split sibling of <child-1-id>. Parent: <parent-id>."
```

(Verify `bd supersede` multi-`--with` behavior during implementation; prefer the native mechanism if supported.)

### Dependency detection

Principle: if one child **produces a contract** the other **consumes**, add a `bd dep`. Otherwise leave siblings parallel.

Heuristics, in order:

1. **Contract producers before consumers:**
   - `.NET/API` → `iOS`, `web` (endpoint before its clients).
   - `Pulumi/infra` → anything that deploys into that infra.
   - `GitHub Actions` → code that relies on a new workflow or secret.
2. **Delete after migrate:** A `delete-worker` child always depends on any non-delete siblings.
3. **Otherwise parallel.** Two UI siblings (iOS + web) consuming an existing API have no dep between them.

Applied via:

```bash
bd dep add <consumer-child-id> <producer-child-id>
```

Autopilot writes its dependency reasoning into each child's notes so a human reviewer can check it:

```
"Depends on <sibling-id> because: iOS client consumes the API endpoint added there."
```

**Cycle or ambiguity:** if autopilot can't articulate a one-sentence reason for a dep, it doesn't add one. Parallel siblings are a safer default than a wrong dep direction.

**Producer/consumer unclear:** mark the parent blocked for human triage. Don't silently mis-split.

### Return behavior

After creating children, linking the parent, adding deps, and running `bd dolt push`, the tick ends:

```
Autopilot: split <parent-id> into <child-1-id>, <child-2-id>, ... — loop will pick up ready children.
```

No code work happens in the split tick. The next cron tick picks up the first unblocked child via `/beads:ready`. Dependent children become ready after their producer closes.

## Consequences

**Easier:**
- Multi-area features and migrations flow through the loop without human intervention.
- Each worker stays in its allowed path; no cross-cutting work on a single branch.
- The supersede/dep links preserve the logical grouping for humans reading the bead history.

**Harder:**
- Autopilot now reasons about bead content, not just classifies by keyword. Mis-splits are possible; mitigation is the "ambiguous → block for human" escape hatch plus the one-sentence dep reasoning.
- Two extra round-trips before first dispatch (split tick → ready tick → dispatch). Acceptable when the loop ticks every few minutes.
- Parent bead is superseded rather than closed, which leaves a slightly different history shape than single-worker beads.

**Unchanged:**
- "One bead per invocation" holds — the split tick is a split, not a dispatch.
- Worker types, allowed paths, merge/test/push validation, and failure handling all remain as today.
