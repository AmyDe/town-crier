# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

**How this file works.** It is the always-loaded layer: a map of the repo plus the rules that must be known *before* acting. Everything else lives one hop away and loads on demand — reach for the deeper layer instead of guessing:

| Layer | Where | How it loads |
|-------|-------|--------------|
| Per-stack coding patterns | `.claude/skills/*/SKILL.md` — compact core + `references/` pulled per topic | Auto-triggers on path/task (see the routing table below) |
| Design rationale | `docs/adr/` (decisions), `docs/memo/` (analysis without a decision) | Read when touching that area |
| Feature designs / specs | GitHub issue bodies — never committed files | `gh issue view <n>` |
| Operational gotchas | Auto-memory (`MEMORY.md` index) | Injected each session |
| Beads workflow | `bd prime` | Injected each session |

## Project Overview

Town Crier is a mobile-first app for monitoring UK local authority planning applications, delivering push notifications to residents, community groups, and property professionals. It uses PlanIt (planit.org.uk) as its primary data provider, with a polling-based ingestion model (see ADR 0006). PlanIt is a free, single-operator service — read the hard call limits below before calling it from a local session.

## PlanIt: Hard Call Limits

PlanIt is a **free service run by one individual** and our sole planning-data provider (ADR 0006). Not hammering it is non-negotiable — but that is a rule about **behaviour, not a daily quota**. There is **no fixed daily call budget**: don't introduce one, don't enforce one, and don't raise findings on the basis of one. (ADR 0041 and ADR 0042 cite a `~1,500 requests/day` figure; that is historical context from when those decisions were made, not a live limit.)

**The deployed poller is not the risk.** It honours `Retry-After` (a 429 is never retried internally — it ends the cycle so the scheduler reschedules), backs off for 2h after a timeout, caps attempts at 4, and sleeps 2s before *every* attempt including retries. That behaviour is what "polite" means here, and it is enforced in `api-go/internal/planit/client.go`.

**A local session is the risk** — a `curl` loop, a throwaway Python script, a "let me just test this quickly" harness. It has none of those brakes and no review. So, binding on any local or agent session calling PlanIt by hand:

- **Never more than 10 requests total in a session.**
- **Never more than one request per 60 seconds.**
- **Never write a loop that hits PlanIt without an explicit sleep of 60s or more between iterations.**

No exceptions for "it's only a few more", for batching, or for running it in parallel. If a task looks like it needs more than 10 calls, **stop and ask** rather than proceeding. Prefer the `httptest`-backed doubles in `api-go/internal/planit/*_test.go` over live calls.

## Business Status

**Town Crier is no longer pre-revenue.** As of 2026-06-29 we have our first arm's-length paying customer. Treat the product as live with real customers.

Operational consequence: the pre-revenue **fix-forward-always, no-rollback** posture is **reversed**. A broken deploy can harm a paying user. Rollback safety, soak/verification gates before promoting, and staged cutovers are back on the table; don't assume "we can just fix forward" is acceptable for anything user-facing. Judge blast radius rather than reaching for blunt cutovers.

## Repository Map

```
/api-go             Go backend. Binaries: cmd/api (HTTP), cmd/worker (background jobs),
                    cmd/pgmigrate, cmd/pgbootstrap. ~40 feature packages under internal/
                    (one feature = one package: watchzones, notifications, polling, planit,
                    subscriptions, sharepage, erasure, digest, apns, offercodes, …);
                    cross-cutting code in internal/platform/ (incl. postgres/pgtest harness).
/cli                Go admin CLI (`tc`).
/mobile/ios         Native iOS app (SwiftUI, MVVM-C). SPM packages under packages/:
                    town-crier-domain, town-crier-data, town-crier-presentation; app shell
                    town-crier-app/, tests town-crier-tests/. xcodegen: project.yml → xcodeproj.
/web                React/TypeScript frontend (Vite). src/features/<Feature>/ slices (Map,
                    WatchZones, Dashboard, onboarding, …) plus api/, auth/, components/, hooks/.
/infra              Pulumi IaC (Go). Mostly two files — environment.go (per-env resources)
                    and shared.go — plus Pulumi.{dev,prod,shared}.yaml stacks.
/docs               adr/ (accepted decisions), memo/ (open analysis), cost-forecast/,
                    product-overview.md.
/scripts            Release + legal helpers (ios-release-notes.sh, sync-legal.sh,
                    check-legal-drift.sh) and wf/ orchestration helpers
                    (worktree-setup.sh, watch-pr.sh, next-version.sh, release-notes.sh).
/.github/workflows  pr-gate, cd-dev, cd-prod, cd-ios-testflight, auto-merge, seo-refresh,
                    dev-container-app-cleanup, ios-capability-ops, legal-drift-check.
/.claude            skills/ (per-stack + task skills), agents/ (worker definitions),
                    PreToolUse hooks (require-bead.sh, require-worktree.sh), worktrees/.
/.beads             bd issue DB (Dolt server mode). Deliberately NO issues.jsonl — see
                    "Beads: Dolt is the sole source of truth" below.
```

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Backend API | Go (net/http, log/slog), Azure Container Apps |
| Database | Azure Database for PostgreSQL Flexible Server + PostGIS via pgx (no ORM) |
| iOS App | Swift, SwiftUI, SwiftData, SPM |
| Infrastructure | Pulumi (Go) |
| CI/CD | GitHub Actions |
| Testing | go test (API, CLI, infra), Swift Testing (iOS), Vitest (web) |

## Data Sources

When looking up user data or entity information, query our own Postgres database first (the `town_crier_prod` database) — not Auth0 or other external identity providers. Auth0 tenants may be deleted or unavailable, and most user data is already stored in Postgres from onboarding.

## Git & Release Workflow

- Never commit directly to main. Always create a feature branch and open a PR, even for small fixes.
- **Create the branch as its own step before pushing.** Don't bundle branch creation and `git push` into one move from `main` — a PreToolUse hook blocks `git push` while the current branch is `main`. Sequence it: `git checkout -b <branch>`, then `git push -u origin <branch>`, then `gh pr create`.
- When git push or tag push is blocked by branch protection hooks, immediately fall back to `gh release create` instead of debugging the hook.
- **User-facing iOS changes get a `Release-Note:` git trailer.** The TestFlight changelog is generated by `scripts/ios-release-notes.sh`, which never shows the raw GitHub release body. For any commit that changes what an iOS user sees or does, add one plain-English line, e.g. `Release-Note: Saved searches now refresh the moment an application changes.` Trailers ship verbatim; commits without one fall back to cleaned `feat`/`fix` subjects. Skip the trailer for backend-only, infra, CI, or refactor commits.

## Release Process

**The iOS beta lane is fastlane.** `cd-ios-testflight` runs `xcodegen generate` then `bundle exec fastlane beta` (`mobile/ios/fastlane/Fastfile`, signing via match). The `.xcodeproj` is generated output — edit `mobile/ios/project.yml`, never the project file.

iOS patch releases keep tripping on the same two snags — bake the checks in:

- **The version train can close.** After an App Store release, the `MARKETING_VERSION` train closes and the next TestFlight upload fails with an altool **409**. The CD beta lane bumps the *build* number, never `MARKETING_VERSION`. On a 409 — or pre-emptively when the last release shipped to the App Store — bump `MARKETING_VERSION` in `mobile/ios/project.yml` and re-release.
- **Anchor "What's New" to the LIVE App Store version**, not the previous tag or latest TestFlight build. Use the `ios-whats-new` skill for store copy. The TestFlight changelog is driven by `Release-Note:` trailers, which a squash-merge can collapse — confirm they survived before releasing.

## Follow-up Checks

Never suggest `/schedule` for future checks (deploy verification, post-merge monitoring). Cloud agents lack the local credentials this repo's checks rely on — `gh`, `az`, `auth0`, `pulumi`, the Postgres endpoint, the Dolt-backed `bd` server — and fail silently. `/loop` runs in the local session with full credentials; it is the only viable follow-up mechanism. Offer `/loop`, never `/schedule`.

## Debugging Guidelines

When diagnosing issues, ask the user for the data source or context before exploring the codebase broadly. Don't assume where data comes from (e.g., Auth0 vs Postgres, OpenTelemetry vs logs).

## Scope Discipline

- When asked only to plan, scope, or rewrite a prompt or spec, do NOT start implementing. Produce the plan and stop for an explicit go-ahead.
- Workers and subagents implement ONLY what their bead or brief describes. If the task seems to imply broader changes — auth, telemetry, logging, anything privacy- or GDPR-sensitive — STOP and flag it rather than commit. Work that has to be reverted costs more than the question would have.
- Explicit holds survive the whole task. If the user said "don't touch X," that applies to every subagent you dispatch too, not just your own edits.

## Testing & CI

When fixing CI failures, always check for ALL root causes before declaring the fix complete. Run the full test suite and verify end-to-end, not just the first failure.

## UI Verification — Agent-Run, No Human in the Loop

Front-end changes are verified live before the work is declared done — but never drive the UI directly from the main/parent session. Screenshots are expensive to load into context, so always dispatch a `model: sonnet` subagent (`Agent` tool) to do the driving and inspect its own screenshots, then have it report back a concise pass/fail summary and any defects found — not the raw images:

- **Web** — subagent drives the `agent-browser` CLI (screenshot paths must be absolute).
- **iOS / Android** — subagent drives the `mobile-mcp` MCP server against the iOS simulator and the Android emulator (AVD `towncrier` on the dev machine): install, launch, tap, type, screenshot.

Never ask the human to click through a UI to confirm a change, and never call `agent-browser` or `mobile-mcp` tools directly from the main session — always through a dispatched subagent.

## Development Commands

### Go API (`/api-go`)

```bash
# Run from api-go/
go build ./...                      # Build
go test ./...                       # All unit tests (hand-written fakes; no Docker)
go vet ./...                        # Static analysis
gofmt -l .                          # List files needing formatting (empty = clean)
make test-integration               # Real Postgres+PostGIS suite — boots a local Docker DB
go test -tags=integration ./...     # Same, against a running DB (TEST_DATABASE_URL); skips cleanly if none
```

The default `go test ./...` excludes the `integration`-tagged real-DB suite, so it needs no Docker. The `go-coding-standards` skill documents the `pgtest` harness API and when real-DB tests are required.

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
cd web && npx vitest run            # Run tests
```

## Coding Standards Skills and Workers

When a bead targets a given tech stack, use the matching skill and worker agent. The mapping is a straight lookup — pick the row, use those tools.

| Tech stack          | Path             | Skill                     | Worker agent           |
|---------------------|------------------|---------------------------|------------------------|
| Go                  | `/api-go`, `/cli` | `go-coding-standards`    | `go-tdd-worker`        |
| iOS / Swift         | `/mobile/ios`    | `ios-coding-standards`    | `ios-tdd-worker`       |
| Android / Kotlin    | `/mobile/android`| `android-coding-standards`| `android-tdd-worker`   |
| Web / React / TS    | `/web`           | `react-coding-standards`  | `react-tdd-worker`     |
| Pulumi infra (Go)   | `/infra`         | `go-coding-standards`     | `pulumi-infra-worker`  |
| GitHub Actions      | `.github/`       | —                         | `github-actions-worker`|
| UI (any platform)   | UI code in any of the above | `design-language` (in addition to the platform skill) | — |

Consult the skill before writing, reviewing, or scaffolding code for that stack. Standard lint configs ship as skill assets (`.golangci.yml` under `go-coding-standards`, `.swiftlint.yml` under `ios-coding-standards`, `.editorconfig` + `detekt.yml` under `android-coding-standards`).

### Worker model policy

Worker agents default to `model: sonnet` (set in each agent's frontmatter) — Sonnet-first with a retry is cheaper than Opus-always. Escalate deliberately:

- **Re-dispatch a bead once with `model: opus`** only after a Sonnet worker fails its pre-flight gates (tests/lint/build) or the PR gate rejects the work.
- **Read-only fan-out** (`Explore`, locating code) can use `model: haiku`; **read-and-summarise** subagents use `model: sonnet`.
- Reserve the premium default model for design/spec sessions; a goal-runner session that only dispatches, watches gates, and merges can itself run on Sonnet.

Do not pin `model:` at the `Agent()` call site for workers; the frontmatter is the single control.

## Key Architectural Constraints

Per-stack architecture and patterns live in that stack's coding-standards skill. Cross-cutting:

- **No ORM** — read and write Postgres through the pgx driver directly. Business logic lives in domain entities and value objects; HTTP handlers are lightweight orchestrators.
- **TDD workflow: Red-Green-Refactor.** Primary unit of test by stack: HTTP handlers and stores (Go); ViewModels and Use Cases (iOS); hooks (web).
- **Hand-written fakes/spies** — no reflection-based mocking libraries.
- **Real-DB integration tests (Go).** Postgres store ports have real-database tests against local PostGIS in Docker (`//go:build integration`, `pgtest` harness) covering spatial/SQL behaviour fakes can't honestly model (`ST_DWithin`, KNN ordering, accurate `COUNT`). Additive to the unit fakes, not a replacement. See ADR 0032 and the `go-coding-standards` skill.
- **Naming:** directories lowercase-hyphenated; Swift types PascalCase, no `I` prefix on protocols, classes `final` by default.

## Legal Documents

Privacy Policy and Terms of Service live as JSON at `api-go/internal/legal/resources/{privacy,terms}.json`, embedded and served via `/v1/legal/{type}`, with a byte-equal iOS mirror. To change legal copy, use the `legal` skill; mechanically: edit the API JSON, run `scripts/sync-legal.sh`, commit both — CI fails on drift (`scripts/check-legal-drift.sh`).

## Specs and Beads

### Philosophy

Beads track *what to do* and *dependencies*. The `how` and `why` of a feature live in **the GitHub issue body** — never in a committed spec file. ([Yegge, Issue #976](https://github.com/gastownhall/beads/issues/976))

**Never commit spec files to the repo.** No `docs/specs/*.md`, no design markdown alongside code, no per-feature plan files — they rot faster than the code and mislead future readers. When a feature needs design context: raise a self-contained GitHub issue with the `file-issue` skill (problem, approach, acceptance criteria, edge cases, test plan), then reference it from the bead description (`GH: https://github.com/<org>/<repo>/issues/123`). Workers read it with `gh issue view <n>`. If a bead is too thin to act on, push the design into the issue, not into a markdown file.

### Beads (Exclusive Issue Tracker)

Use `bd` for ALL task tracking. Do NOT use TodoWrite, TaskCreate, or markdown files.

- Run `bd prime` to load workflow context (commands, session protocol).
- Do NOT use `bd edit` — it opens an interactive editor that blocks agents.
- `bd dolt push` runs automatically on `git push` via the bd pre-push hook (install once per clone with `bd hooks install`).
- **End every commit subject with `(<bead-id>)`** (e.g. `fix: expire stale sessions (tc-a1b2)`) so `bd doctor` can detect orphan beads.
- **Write handoff notes before stopping** — beads survive compaction; conversation history doesn't. Use `COMPLETED: … IN PROGRESS: … NEXT: … BLOCKER: … KEY DECISIONS: …` (overwrite, don't append).
- **Link side-quest work with `discovered-from`** — file a new bead and `bd dep add <new> <current> --type=discovered-from`.

### Bead-First Rule

**Every code change requires a bead — no exceptions.** Even a one-line typo fix. Before editing any source file: `bd create --title="<change>" --type=task --priority=3`, then `bd update <id> --claim`; `bd close <id>` when done. A PreToolUse hook (`.claude/require-bead.sh`) blocks Write/Edit on code files when no bead is in_progress. Do not work around it.

### Worktree-First Rule

**All code changes happen in a worktree — never in the main working tree.** Parallel conversations editing the main tree conflict.

- **`scripts/wf/worktree-setup.sh <name> [--branch <branch>]` runs the whole recipe**: resets local main to `origin/main`, runs `bd worktree create` (never raw `git worktree add`), applies the two bd workarounds (GH#3421 port symlink, beads#3593 chmod), resets the worktree to `origin/main`, and prints the path. Always prefer it; the script's header documents the manual fallback. Remove the workarounds when the upstream bd fixes ship.
- Then `EnterWorktree path: "<printed path>"`, make changes there, and use `/ship` or `ExitWorktree` when done; `bd worktree remove <name>` for cleanup (name, not path).
- PreToolUse hooks enforce this: Write/Edit on code files is blocked outside a worktree, and raw `git worktree add` is blocked in favour of `bd worktree create`. Do not work around them.
- **The orchestrator creates the worktree, not the subagent.** Dispatch workers with the worktree path already in hand — keeps the lifecycle (create, verify, remove) in one place.

### Cleanup Discipline

Keep the working set (open + in-progress) under ~200 issues.

- **`bd flatten --force`** — squash Dolt commit history when `bd` gets sluggish (main speed lever; prefer over `bd compact`, whose squash can fail on a churned DB).
- **`bd admin compact`** — semantic decay of old closed issues; run ~quarterly: `bd compact --analyze --json` → write summaries → `bd compact --apply --id <id> --summary -`.
- Stay on **stable bd 1.0.4**; do not upgrade to pre-releases (1.0.5's schema migration corrupts this DB).

## Beads: Dolt is the sole source of truth (DO NOT re-add issues.jsonl)

bd 1.0.4 server mode re-imports `.beads/issues.jsonl` on every command and can clobber just-made writes (gastownhall/beads #3849). The fix in place: the jsonl is **removed** from `.beads/` (archived at `~/.beads-archive/town-crier/`), `export.auto = false` and `export.git-add = false` in `.beads/config.yaml`, and only the pinned `~/.local/bin/bd` 1.0.4 exists (never `brew install beads`). Sync is **Dolt only**: `bd dolt push` / `bd dolt pull` against the DoltHub remote (`amyde/town-crier`); a fresh clone hydrates via `bd dolt pull`, never a jsonl import.

**Rules:** never re-create `.beads/issues.jsonl`, never re-enable `export.auto`/`export.git-add`, never run `bd export` to the default path, never re-track the jsonl, never upgrade off 1.0.4. Full root cause + recovery recipe: `docs/memo/0011-beads-dolt-write-thrash-root-cause.md` and auto-memory `project_bd_thrash_and_105_breakage`.

After a squash-merge, never `git pull --rebase` local main onto origin/main. Use `git fetch origin && git reset --hard origin/main` instead.

When re-running `bd init`, `bd init --server`, or `bd setup claude`, always pass `--skip-agents` — otherwise bd re-inserts its own integration block, which duplicates and drifts from this section.

## CLI-First Policy

ALWAYS use installed CLI tools before asking the user to do something manually — no web consoles or manual steps when a CLI can do it:

| Tool | CLI | Use for |
|------|-----|---------|
| GitHub | `gh` | Repos, PRs, issues, releases, Actions |
| Azure | `az` | Resource management, deployments, configuration |
| Auth0 | `auth0` | Tenant management, apps, APIs, users |
| Cloudflare | `cloudflared` | Tunnels, DNS, access, routing |
| Pulumi | `pulumi` | Infrastructure provisioning, stacks, config |

If a command fails or requires interactive auth, only then ask the user to intervene.

### Deployments — PR-Only Policy

NEVER deploy code directly using `az`, `pulumi`, or any other CLI. ALL code changes ship via pull requests and deploy through CI/CD (GitHub Actions). Direct deployments bypass review, testing, and audit trails.

## Shell & Tooling

- **Non-interactive flags always.** Some systems alias `cp`/`mv`/`rm` to `-i`, hanging agents on prompts: use `cp -f`, `mv -f`, `rm -f`, `rm -rf`. Same principle for CLI fix/format commands — pass `--yes` or equivalent; never assume interactive prompts will work.
- **RTK rewrites `rg`/`grep`.** A shell hook proxies these through RTK, which can mangle output and make the Grep tool come back empty. If a Grep result looks wrong or empty, fall back to plain `grep`/`rg` in Bash (or `rtk proxy <cmd>`).
- **Bash CWD persists between calls.** A stale `cd` into a worktree makes a later reset/create hit the wrong tree. Anchor every cleanup/sync/create to the repo root with `git -C <repo-root> …` or an absolute path.
- **Escape backticks in bead notes.** A `` ` `` in `bd update --notes "…"` triggers shell command substitution. Wrap notes in single quotes.

## Session Completion Checklist

When ending a work session where code was changed:

1. **File issues for remaining work** — `bd create` for anything needing follow-up (`--type=discovered-from` when it came out of the current task).
2. **Update bead notes on the in-progress bead** — overwrite with the `COMPLETED/IN PROGRESS/NEXT/BLOCKER/KEY DECISIONS` shape so the next session can resume with zero conversation context.
3. **Run quality gates** — tests, linters, builds.
4. **Update issue status** — `bd close` finished work, update in-progress items.
5. **Sync and push** — from a feature branch, `git pull --rebase && git push`; on main, sync only via `git fetch origin && git reset --hard origin/main` (never rebase-pull main after a squash-merge). The pre-push hook runs `bd dolt push` automatically.
6. **Verify** — `git status` shows "up to date with origin".

## Memory Management

Proactively update Claude Code memory (`~/.claude/projects/.../memory/`) whenever you learn something noteworthy — new architectural decisions or constraints, user preferences or workflow patterns, project context useful in future sessions, corrections or feedback. Do not wait to be asked.

## ADRs and Memos

- **ADR** (`docs/adr/NNNN-title.md`) for any major architectural decision: adopting or rejecting a technology, choosing a structural pattern, making a significant trade-off, or reversing a prior decision. Copy `docs/adr/0000-template.md` (Status / Context / Decision / Consequences).
- **Memo** (`docs/memo/NNNN-title.md`) for analysis that hasn't produced a decision yet: explored trade-offs, future migration paths, options evaluated with no action taken. Copy `docs/memo/0000-template.md` (Status / Question / Analysis / Options Considered / Recommendation). A memo graduates to an ADR when a decision lands (mark it `Superseded by ADR NNNN`).
