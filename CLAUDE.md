# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Town Crier is a mobile-first app for monitoring UK local authority planning applications, delivering push notifications to residents, community groups, and property professionals. It uses PlanIt (planit.org.uk) as its primary data provider, with a polling-based ingestion model (see ADR 0006).

## Monorepo Structure

```
/api          # .NET 10 backend (Hexagonal / Ports & Adapters)
/mobile/ios   # Native iOS app (Clean Architecture / MVVM-C)
/web          # React/TypeScript frontend
/infra        # Pulumi IaC (.NET 10 / C#)
/docs/adr     # Architecture Decision Records
/docs/specs   # Feature specs referenced by beads
```

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Backend API | .NET 10 (ASP.NET Core), Native AOT, Azure Container Apps |
| Database | Azure Cosmos DB (Serverless) via Cosmos DB SDK (no ORM) |
| iOS App | Swift, SwiftUI, SwiftData, SPM |
| Infrastructure | Pulumi (C#/.NET 10) |
| CI/CD | GitHub Actions |
| Testing | TUnit (.NET), XCTest (iOS) |

## Data Sources

When looking up user data or entity information, query Cosmos DB first — not Auth0 or other external identity providers. Auth0 tenants may be deleted or unavailable, and most user data is already stored in Cosmos from onboarding.

## Git & Release Workflow

- Never commit directly to main. Always create a feature branch and open a PR, even for small fixes.
- When git push or tag push is blocked by branch protection hooks, immediately fall back to `gh release create` instead of debugging the hook.

## Debugging Guidelines

When diagnosing issues, ask the user for the data source or context before exploring the codebase broadly. Don't assume where data comes from (e.g., Auth0 vs Cosmos, OpenTelemetry vs logs).

## Testing & CI

When fixing CI failures, always check for ALL root causes before declaring the fix complete. Run the full test suite and verify end-to-end, not just the first failure.

## Development Commands

### .NET API (`/api`)

```bash
dotnet build                        # Build
dotnet test                         # Run all tests
dotnet test --filter "TestName"     # Run a single test
dotnet format --verify-no-changes   # Check formatting
dotnet format                       # Auto-fix formatting
```

### iOS (`/mobile/ios`)

```bash
swift build                         # Build
swift test                          # Run all tests
swiftlint lint --strict             # Lint
swift-format format --in-place --recursive .  # Auto-format
```

### Web (`/web`)

```bash
cd web && npm run dev               # Vite dev server with hot reload
cd web && npm run build             # Production build to /web/dist
cd web && npx tsc --noEmit          # Type check without emitting
cd web && npx vitest run            # Run tests (when Vitest is added)
```

## Coding Standards Skills

This repo has Claude Code skills that are **auto-triggered** when writing code:

- `dotnet-coding-standards` — Auto-invoked when writing, reviewing, or scaffolding any C# code in `/api`. Covers DDD, CQRS, TUnit testing, Cosmos DB SDK, and Native AOT patterns.
- `ios-coding-standards` — Auto-invoked when writing, reviewing, or scaffolding any Swift code in `/mobile/ios`. Covers MVVM-C, protocol-oriented design, XCTest, and Swift Concurrency.
- `react-coding-standards` — Auto-invoked when writing, reviewing, or scaffolding any React/TypeScript code in `/web`. Covers feature-sliced clean architecture, CSS Modules with design tokens, hooks-as-ViewModels, Vitest + Testing Library, and domain-layer purity.
- `design-language` — Auto-invoked when creating or modifying any UI code, colors, themes, or visual components. Defines the cross-platform design system (colors, typography, spacing, components) with light, dark, and OLED dark theme support.

Skills are defined in `.claude/skills/` as `SKILL.md` files.

## Key Architectural Constraints

### Native AOT (.NET)
All .NET code must be Native AOT-compatible:
- No reflection (`typeof(T).GetProperties()`, `Activator.CreateInstance`)
- System.Text.Json with `[JsonSerializable]` source generators — no Newtonsoft.Json
- No dynamic assembly loading or runtime code generation
- No EF Core or Dapper — use Cosmos DB SDK directly

### CQRS (Manual Dispatch)
Command/Query handlers are manually dispatched — no MediatR or similar libraries. Keep the dependency tree minimal.

### DDD (Rich Domain Models)
Business logic lives in domain entities and value objects, not in handlers. Handlers are lightweight orchestrators only.

### Testing
- TDD workflow: Red-Green-Refactor
- Handlers are the primary unit of test (.NET); ViewModels and Use Cases for iOS
- Builder pattern for test data (.NET); static extension fixtures for iOS
- Manual fakes/spies — no reflection-based mocking libraries

## Naming Conventions

- **Directory/project names:** `town-crier.*` (lowercase, hyphenated)
- **C# namespaces:** `TownCrier.*` (PascalCase)
- **Swift types:** PascalCase; no `I` prefix on protocols (use `...able`, `...Service`, `...Repository`)
- **C# classes:** `sealed` by default; Swift classes: `final` by default

## Specs and Beads

### Philosophy

Beads track *what to do* and *dependencies*. Specs capture *how* and *why*. Keep beads lightweight — one-line descriptions referencing spec files. ([Yegge, Issue #976](https://github.com/gastownhall/beads/issues/976))

### Specs Convention

Feature specifications live in `docs/specs/<topic>.md`. When breaking down a plan into beads, create the spec file first, then create beads that reference it:

```bash
bd create --title="Implement auth flow" --description="Spec: docs/specs/auth-flow.md#phase-1" --type=task --priority=2
```

### Beads (Exclusive Issue Tracker)

Use `bd` for ALL task tracking. Do NOT use TodoWrite, TaskCreate, or markdown files.

- Run `bd prime` to load workflow context (commands, session protocol)
- Do NOT use `bd edit` — it opens an interactive editor that blocks agents
- `bd dolt push` runs automatically on `git push` via the bd pre-push hook (install once per clone with `bd hooks install`). No manual sync needed.
- **End every commit subject with `(<bead-id>)`** (e.g. `fix: expire stale sessions (tc-a1b2)`). This enables `bd doctor` to detect orphan beads — work committed without closing its issue.
- **Write handoff notes before stopping** — beads survive compaction; conversation history doesn't. Use the structured shape `COMPLETED: … IN PROGRESS: … NEXT: … BLOCKER: … KEY DECISIONS: …` (overwrite, don't append). Worker skills enforce this on their last commit.
- **Link side-quest work with `discovered-from`** — when you spot unrelated work during a task, file a new bead and run `bd dep add <new> <current> --type=discovered-from`. Preserves the "why was this filed?" trail.

### Bead-First Rule

**Every code change requires a bead — no exceptions.** Even a one-line typo fix gets a bead. Beads are the ledger of all work done; if it's not in a bead, it didn't happen.

Before editing any source file (`.swift`, `.cs`, `.ts`, `.tsx`, `.css`, `.csproj`):
1. `bd create --title="<describe the change>" --type=task --priority=3`
2. `bd update <id> --claim`
3. Make your changes
4. `bd close <id>`

A PreToolUse hook (`.claude/require-bead.sh`) enforces this — Write/Edit on code files is blocked when no bead is in_progress. Do not try to work around it.

### Worktree-First Rule

**All code changes must happen in a worktree — never in the main working tree.** Multiple conversations often run in parallel; editing the main tree causes conflicts.

Before editing code, create and enter an isolated worktree:
1. `bd worktree create <name>` (optionally `--branch <branch>`) — wraps `git worktree add`, keeps the shared beads DB visible, and avoids beads bug [GH#3311](https://github.com/gastownhall/beads/issues/3311).
2. `EnterWorktree path: "<path printed by the command>"` to switch the session into it.
3. Make your changes within the worktree.
4. Use the `/ship` skill or `ExitWorktree` when done; `bd worktree remove <name>` for cleanup (has safety checks for uncommitted/unpushed work).

A PreToolUse hook (`.claude/require-worktree.sh`) enforces this — Write/Edit on code files is blocked when the session is not inside a worktree. A second hook blocks raw `git worktree add` and directs you to `bd worktree create`. Do not try to work around them.

**Who creates the worktree?** The orchestrator (top-level agent), not the subagent. Dispatch workers with the worktree path already in hand — this matches how `autopilot` and the TDD workers are wired and keeps worktree lifecycle (create, verify, remove) in one place.

### Superpowers Workflow Integration

When using the superpowers brainstorm → plan → execute workflow, beads are the task ledger:

1. **After `writing-plans`:** ALWAYS run `plan-to-beads` before executing. This converts plan tasks into beads and is a required bridge step — not optional.
2. **Instead of TodoWrite:** Use beads for all task state. The superpowers skills say "Create TodoWrite" and "Mark task complete in TodoWrite" — replace these with:
   - Session start: `bd list --status=open` to see the task ledger
   - Starting a task: `bd update <id> --status=in_progress`
   - Completing a task: `bd close <id>`
3. **Subagent implementers** should `bd update <id> --claim` at the start of their task and `bd close <id>` when done.

This applies to both `superpowers:subagent-driven-development` and `superpowers:executing-plans`. User instructions take precedence over skill instructions per the superpowers contract.

### Cleanup Discipline

Keep the working set under ~200 issues. Run `bd cleanup` regularly and compact closed issues with `bd compact`.

## CLI-First Policy

ALWAYS use installed CLI tools to complete tasks before asking the user to do something manually. Do not suggest the user run commands, visit web consoles, or perform manual steps when a CLI tool can accomplish the same thing.

### Installed CLI tools

| Tool | CLI | Use for |
|------|-----|---------|
| GitHub | `gh` | Repos, PRs, issues, releases, Actions, etc. |
| Azure | `az` | Resource management, deployments, configuration |
| Auth0 | `auth0` | Tenant management, apps, APIs, rules, users |
| Cloudflare | `cloudflared` | Tunnels, DNS, access, routing |
| Pulumi | `pulumi` | Infrastructure provisioning, stack management, config |

When a task involves any of these services, use the corresponding CLI directly. If a command fails or requires interactive auth, only then ask the user to intervene.

### Deployments — PR-Only Policy

NEVER deploy code directly using `az`, `pulumi`, or any other CLI tool. ALL code changes MUST be shipped via pull requests and deployed through CI/CD (GitHub Actions). Direct deployments bypass review, testing, and audit trails.

## Beads JSONL Merge Driver (one-time setup)

`.beads/issues.jsonl` is a derived Dolt snapshot rewritten on every bead mutation, so any two branches that touched beads will conflict on merge/rebase. `.gitattributes` marks the file with `merge=theirs`. The driver itself is per-clone — register it once:

```bash
git config --local merge.theirs.driver 'cp -f "%B" "%A"'
git config --local merge.theirs.name 'always take incoming version'
```

After squash-merge, never `git pull --rebase` local main onto origin/main — the local auto-export commits will conflict with the squashed version. Use `git fetch origin && git reset --hard origin/main` instead.

## Shell Commands — Non-Interactive Mode

Always use non-interactive flags with file operations to avoid hanging on confirmation prompts (some systems alias `cp`, `mv`, `rm` to include `-i`):

```bash
cp -f source dest               # NOT: cp source dest
mv -f source dest               # NOT: mv source dest
rm -f file                      # NOT: rm file
rm -rf directory                # NOT: rm -r directory
```

Always use `--yes` or equivalent non-interactive flags when running CLI fix/format commands in Bash. Never assume interactive prompts will work.

## Session Completion Checklist

When ending a work session where code was changed:

1. **File issues for remaining work** — `bd create` for anything needing follow-up (link with `--type=discovered-from` when it came out of the current task).
2. **Update bead notes on the in-progress bead** — overwrite with `COMPLETED: … IN PROGRESS: … NEXT: … BLOCKER: … KEY DECISIONS: …` so the next session (or post-compaction you) can resume with zero conversation context.
3. **Run quality gates** — tests, linters, builds.
4. **Update issue status** — `bd close` finished work, update in-progress items.
5. **Push** —
   ```bash
   git pull --rebase
   git push   # pre-push hook runs bd dolt push automatically (requires `bd hooks install`)
   ```
6. **Verify** — `git status` should show "up to date with origin".

## Memory Management

Proactively update Claude Code memory (`~/.claude/projects/.../memory/`) whenever you learn something noteworthy during a session. This includes:

- New architectural decisions or constraints
- User preferences or workflow patterns
- Project context that would be useful in future sessions
- Corrections or feedback from the user

Do not wait to be asked — if it seems worth remembering, save it.

## Architecture Decision Records (ADRs)

Record an ADR in `/docs/adr/` for any major architectural decision. Use the format `NNNN-title.md` (zero-padded sequence number). An ADR should be written whenever:

- A new technology, framework, or library is adopted (or explicitly rejected)
- A structural pattern is chosen (e.g., CQRS, event sourcing, specific auth flow)
- A significant trade-off is made (e.g., performance vs. simplicity, build vs. buy)
- An existing architectural decision is revisited or reversed

Use this template:

```markdown
# NNNN. Title

Date: YYYY-MM-DD

## Status

Proposed | Accepted | Deprecated | Superseded by [NNNN](NNNN-title.md)

## Context

What is the issue that we're seeing that is motivating this decision or change?

## Decision

What is the change that we're proposing and/or doing?

## Consequences

What becomes easier or more difficult to do because of this change?
```

## Technical Memos

Record a memo in `/docs/memo/` to capture analysis, exploration, or discussion that hasn't resulted in a decision yet. Use the format `NNNN-title.md` (zero-padded sequence number). A memo should be written whenever:

- A significant technical question has been explored with trade-offs analysed
- A future migration path or scaling concern has been discussed
- Options have been evaluated but no action is being taken yet

A memo can graduate to an ADR when a decision is made. Use status `Superseded by ADR NNNN` when this happens.

Use this template:

```markdown
# NNNN. Title

Date: YYYY-MM-DD

## Status

Open | Superseded by ADR [NNNN](../adr/NNNN-title.md) | Resolved (no action)

## Question

What prompted this analysis?

## Analysis

What we explored and found.

## Options Considered

The paths forward with trade-offs.

## Recommendation

Current thinking, even if no decision has been made.
```

## Style Enforcement Assets

Standard linting/formatting configs are bundled with their respective skills:
- `.claude/skills/dotnet-coding-standards/assets/.editorconfig` and `Directory.Build.props` (SonarAnalyzer, StyleCop, warnings-as-errors)
- `.claude/skills/ios-coding-standards/assets/.swiftlint.yml` (force cast/try/unwrap as errors)


<!-- BEGIN BEADS INTEGRATION v:2 profile:minimal -->
## Beads Reference

Run `bd prime` for the current, AI-optimized workflow context (per [beads ADR-0001](https://github.com/gastownhall/beads/blob/main/claude-plugin/skills/beads/adr/0001-bd-prime-as-source-of-truth.md), that is the canonical source of truth — do not duplicate it here). Run `bd <command> --help` for specific usage.

Rules:
- `bd` for ALL tracking (not TodoWrite/TaskCreate).
- `bd remember` for persistent knowledge across sessions (not MEMORY.md).
- `bd doctor` when things feel wrong (stuck lock, orphan bead, worktree drift) — it diagnoses and `bd doctor --fix` remediates.
<!-- END BEADS INTEGRATION -->
