# CLAUDE.md

Guidance for Claude Code in this repository. This file is the always-loaded layer: a repo map plus the rules you must know *before* acting. Everything else loads on demand; reach for it instead of guessing:

| Layer | Where | How it loads |
|-------|-------|--------------|
| Per-stack coding patterns | `.claude/skills/*/SKILL.md` (compact core + per-topic `references/`) | Auto-triggers on path/task (routing table below) |
| Design rationale | `docs/adr/` (decisions), `docs/memo/` (analysis without a decision) | Read when touching that area |
| Feature designs / specs | GitHub issue bodies, never committed files | `gh issue view <n>` |
| Operational gotchas | Auto-memory (`MEMORY.md` index) | Injected each session |
| Beads workflow | `bd prime` | Injected each session |

## Project and Business Status

Town Crier is a mobile-first app that monitors UK local authority planning applications and pushes notifications to residents, community groups and property professionals.

**No longer pre-revenue.** First arm's-length paying customer landed 2026-06-29; treat the product as live. The old **fix-forward-always, no-rollback** posture is **reversed**: a broken deploy can harm a paying user, so rollback safety, soak/verification gates before promoting, and staged cutovers are back on the table. Never assume "just fix forward" is fine for anything user-facing; judge blast radius instead of reaching for blunt cutovers.

## PlanIt and Polling

**MUST:** Before you design, change, debug, review or discuss anything that touches PlanIt or polling, read [`POLLING.md`](POLLING.md). That includes calling PlanIt by hand, even once. It holds the hard call limits for local sessions, the facts about PlanIt, and the polling requirements.

## Repository Map and Stack

```
/api-go             Go backend (net/http, log/slog) on Azure Container Apps. Binaries: cmd/api (HTTP),
                    cmd/worker (background jobs), cmd/pgmigrate, cmd/pgbootstrap. ~40 feature packages
                    under internal/ (one feature = one package: watchzones, notifications, polling,
                    planit, subscriptions, sharepage, erasure, digest, apns, offercodes, …);
                    cross-cutting code in internal/platform/ (incl. postgres/pgtest harness).
                    DB: Azure Database for PostgreSQL Flexible Server + PostGIS via pgx (no ORM).
/cli                Go admin CLI (`tc`).
/mobile/ios         Native iOS (Swift, SwiftUI, SwiftData, MVVM-C). SPM packages under packages/:
                    town-crier-domain, town-crier-data, town-crier-presentation; app shell
                    town-crier-app/, tests town-crier-tests/. xcodegen: project.yml → xcodeproj.
/web                React/TypeScript (Vite). src/features/<Feature>/ slices (Map, WatchZones,
                    Dashboard, onboarding, …) plus api/, auth/, components/, hooks/.
/infra              Pulumi IaC (Go). Mostly environment.go (per-env resources) and shared.go,
                    plus Pulumi.{dev,prod,shared}.yaml stacks.
/docs               adr/, memo/, cost-forecast/, product-overview.md.
/scripts            Release + legal helpers (ios-release-notes.sh, sync-legal.sh, check-legal-drift.sh);
                    wf/ orchestration (worktree-setup.sh, watch-pr.sh, next-version.sh, release-notes.sh).
/.github/workflows  pr-gate, cd-dev, cd-prod, cd-ios-testflight, auto-merge, seo-refresh,
                    dev-container-app-cleanup, ios-capability-ops, legal-drift-check.
/.claude            skills/, agents/ (worker definitions), PreToolUse hooks (require-bead.sh,
                    require-worktree.sh), worktrees/.
/.beads             bd issue DB (Dolt server mode). Deliberately NO issues.jsonl (see Beads below).
```

Testing: go test (API, CLI, infra), Swift Testing (iOS), Vitest (web). CI/CD: GitHub Actions.

## Data Sources and Debugging

- Look up user/entity data in our own Postgres (`town_crier_prod`) first, not Auth0 or other external IdPs. Auth0 tenants may be deleted or unavailable, and onboarding already stores most user data in Postgres.
- When diagnosing, ask the user for the data source or context before exploring the codebase broadly. Don't assume where data comes from (Auth0 vs Postgres, OpenTelemetry vs logs).

## Git, PRs and Releases

- Never commit directly to main; always branch and open a PR, even for small fixes.
- **Create the branch as its own step.** A PreToolUse hook blocks `git push` while on `main`, so sequence it: `git checkout -b <branch>`, `git push -u origin <branch>`, `gh pr create`.
- If branch protection hooks block a git push or tag push, fall back to `gh release create` immediately instead of debugging the hook.
- After a squash-merge, never `git pull --rebase` local main; use `git fetch origin && git reset --hard origin/main`.
- **User-facing iOS commits get a `Release-Note:` trailer**: one plain-English line, e.g. `Release-Note: Saved searches now refresh the moment an application changes.` `scripts/ios-release-notes.sh` builds the TestFlight changelog from these verbatim (never from the raw GitHub release body); commits without one fall back to cleaned `feat`/`fix` subjects. Skip it for backend-only, infra, CI or refactor commits. A squash-merge can collapse trailers; confirm they survived before releasing.

### PR merge: no auto-merge, Claude-routine triage

The **Auto-merge** workflow (`.github/workflows/auto-merge.yml`) is deliberately disabled (`disabled_manually`, confirmed 2026-07-29), not broken. Claude routines watch open PRs, triage CodeRabbit comments, and merge. A green PR Gate means **ready for triage**, not **ready to merge**. Do not re-enable Auto-merge.

- **Automated or scheduled context** (cron routine, unattended `/loop`, any session not driven live by the user): never merge, even with all checks green. Stop at "PR Gate passed". Scheduled work can land when the user can't test it; that is exactly what this control is for.
- **Interactive local session:** a human-directed merge is a judgment call. Ask first by default (live product, paying customers); the user may say go ahead for a small, well-tested change.
- The `ship` skill stops at "PR Gate passed" and does not merge.

### iOS releases

- **The beta lane is fastlane.** `cd-ios-testflight` runs `xcodegen generate` then `bundle exec fastlane beta` (`mobile/ios/fastlane/Fastfile`, signing via match). The `.xcodeproj` is generated; edit `mobile/ios/project.yml`, never the project file.
- **The version train can close.** After an App Store release the `MARKETING_VERSION` train closes and the next TestFlight upload fails with altool **409**. CD bumps only the *build* number. On a 409, or pre-emptively when the last release shipped to the App Store, bump `MARKETING_VERSION` in `mobile/ios/project.yml` and re-release.
- **Anchor "What's New" to the LIVE App Store version**, not the previous tag or latest TestFlight build. Use the `ios-whats-new` skill.

## Deployments, CLIs and Follow-ups

- **PR-only deploys.** NEVER deploy with `az`, `pulumi` or any other CLI. All code ships via PR and deploys through GitHub Actions; direct deploys bypass review, testing and audit trails.
- **CLI-first.** Use installed CLIs before asking the user to do anything manually or in a web console: `gh` (repos, PRs, issues, releases, Actions), `az` (resources, deployments, config), `auth0` (tenant, apps, APIs, users), `cloudflared` (tunnels, DNS, access, routing), `pulumi` (provisioning, stacks, config). Ask the user only when a command fails or needs interactive auth.
- **Follow-up checks use `/loop`, never `/schedule`.** Cloud agents lack the local credentials these checks need (`gh`, `az`, `auth0`, `pulumi`, the Postgres endpoint, the Dolt-backed `bd` server) and fail silently.
- **Exception: telemetry monitoring routines** run in the `town-crier-monitor` cloud environment (`env_01D7ocPoeeKVwFGbefgM7xpM`). Its only secret is an **API credential** (OAuth 2.0 client credentials for `sp-claude-monitor`, scope `https://api.loganalytics.io/.default`) that the agent proxy injects on requests to `api.loganalytics.io`; the agent never sees it. Query with `curl -X POST https://api.loganalytics.io/v1/workspaces/842645cf-1439-4a2b-80e8-54bd02e326f9/query -d '{"query":"…"}'`. No `az`, no environment variables, no setup script, network allowlist empty.
  - The principal holds only Log Analytics Data Reader on `log-town-crier-shared` with an ABAC table allowlist (`monitorTableCondition` in `infra/shared.go`: `AppRequests`, `AppDependencies`, `AppExceptions`, `AppAvailabilityResults`). Other tables return **HTTP 200 with zero rows**, not an error, so never read an empty result from them as healthy. Extend the allowlist only with tables proven free of personal data; never widen the role or scope.
  - Attach **no repository** to monitoring routines: the GitHub proxy acts as the owner's account and log content is user-controlled, so a prompt injection could push or merge. No repo also means no beads and no repo hooks.
  - Report by push notification through `Claude_Code_Remote` only; clear every other connector after creating the routine. Monitoring routines diagnose and notify, they never fix, deploy or merge.
  - The client secret expires 2027-01-03. Rotate with `az ad app credential reset --id aacec7b3-8e64-4005-9042-b6233cf984c5`, then delete and re-add the environment's API credential (it cannot be edited).
  - CD can manage only this one role assignment on the shared workspace (conditional RBAC admin, granted by hand). Any other new role assignment in the shared stack 403s in cd-dev until CI is granted access first.

## Scope Discipline

- Asked only to plan, scope, or rewrite a prompt/spec? Do NOT implement. Produce it and stop for an explicit go-ahead.
- Workers and subagents implement ONLY what their bead or brief describes. If the task seems to imply broader changes (auth, telemetry, logging, anything privacy- or GDPR-sensitive), STOP and flag it rather than commit. A revert costs more than the question.
- Explicit holds survive the whole task. "Don't touch X" binds every subagent you dispatch too.

## Code Comments

**MUST, unconditional**, for every stack, test and production code, and every session, worker and subagent. Leave a comment ONLY when it:

1. **Documents a public contract.** A doc comment on an API other code relies on across a package or module boundary: exported Go identifiers, Swift `public`/`open` declarations, exported TypeScript and Kotlin APIs, HTTP endpoints, CLI flags, wire or storage formats. State the contract (inputs, guarantees, errors) and nothing else.
2. **Explains genuinely non-standard behaviour.** The *why* behind code a competent reader would otherwise "fix": an upstream-bug workaround, a deliberate break from the obvious approach, a non-obvious invariant, ordering or limit. One or two sentences.

Everything else is banned: restating or narrating the code; history notes (what it used to do, which PR or bead changed it; git holds that); section banners, commented-out code, speculative TODOs; long explanations where one sentence carries the point.

**Remove verbose comments from code you touch.** When you edit a function, type, test or block, delete or cut every comment in it that fails the bar. This is part of the change, not scope creep, and overrides "implement ONLY what the bead describes" for comments inside edited code. Do not sweep code you did not otherwise change.

## Testing, CI and UI Verification

- When fixing CI, find ALL root causes before declaring done: run the full suite and verify end-to-end, not just the first failure.
- **Front-end changes are verified live before the work is declared done, by an agent, never by the human and never from the main session.** Screenshots are expensive in context, so dispatch a `model: sonnet` subagent (`Agent` tool) to drive the UI, inspect its own screenshots, and report a concise pass/fail plus defects, not raw images.
  - **Web:** `agent-browser` CLI (screenshot paths must be absolute).
  - **iOS / Android:** `mobile-mcp` against the iOS simulator and the Android emulator (AVD `towncrier` on the dev machine): install, launch, tap, type, screenshot.

## Development Commands

```bash
# Go — run from api-go/
go build ./...                      # Build
go test ./...                       # Unit tests (hand-written fakes; excludes integration tag, no Docker)
go vet ./...                        # Static analysis
gofmt -l .                          # Files needing formatting (empty = clean)
make test-integration               # Real Postgres+PostGIS suite; boots a local Docker DB
go test -tags=integration ./...     # Same, against a running DB (TEST_DATABASE_URL); skips cleanly if none

# iOS — run from mobile/ios/
swift build && swift test
swiftlint lint --strict
swift-format format --in-place --recursive .

# Web
cd web && npm run dev               # Vite dev server, hot reload
cd web && npm run build             # Production build to /web/dist
cd web && npx tsc --noEmit          # Type check
cd web && npx vitest run            # Tests
```

## Coding Standards Skills and Workers

When a bead targets a stack, use its row. Consult the skill before writing, reviewing or scaffolding code for that stack.

| Tech stack          | Path             | Skill                     | Worker agent           |
|---------------------|------------------|---------------------------|------------------------|
| Go                  | `/api-go`, `/cli` | `go-coding-standards`    | `go-tdd-worker`        |
| iOS / Swift         | `/mobile/ios`    | `ios-coding-standards`    | `ios-tdd-worker`       |
| Android / Kotlin    | `/mobile/android`| `android-coding-standards`| `android-tdd-worker`   |
| Web / React / TS    | `/web`           | `react-coding-standards`  | `react-tdd-worker`     |
| Pulumi infra (Go)   | `/infra`         | `go-coding-standards`     | `pulumi-infra-worker`  |
| GitHub Actions      | `.github/`       | —                         | `github-actions-worker`|
| UI (any platform)   | UI code in any of the above | `design-language` (plus the platform skill) | — |

Lint configs ship as skill assets: `.golangci.yml` (go), `.swiftlint.yml` (ios), `.editorconfig` + `detekt.yml` (android).

**Worker model policy.** Workers default to `model: sonnet` via their frontmatter, the single control; never pin `model:` at the `Agent()` call site for workers. Sonnet-first with a retry is cheaper than Opus-always.

- Re-dispatch a bead once with `model: opus` only after a Sonnet worker fails its pre-flight gates (tests/lint/build) or the PR gate rejects the work.
- Read-only fan-out (`Explore`, locating code) can use `model: haiku`; read-and-summarise subagents use `model: sonnet`.
- Reserve the premium default model for design/spec sessions; a goal-runner that only dispatches, watches gates and merges can run on Sonnet.

## Key Architectural Constraints

Per-stack patterns live in the stack's skill. Cross-cutting:

- **No ORM:** Postgres through pgx directly. Business logic in domain entities and value objects; HTTP handlers are thin orchestrators.
- **TDD, Red-Green-Refactor.** Primary unit of test: HTTP handlers and stores (Go); ViewModels and Use Cases (iOS); hooks (web).
- **Hand-written fakes/spies**, no reflection-based mocking libraries.
- **Real-DB integration tests (Go).** Postgres store ports also get tests against local PostGIS in Docker (`//go:build integration`, `pgtest` harness) for spatial/SQL behaviour fakes can't honestly model (`ST_DWithin`, KNN ordering, accurate `COUNT`). Additive to unit fakes. See ADR 0032 and `go-coding-standards` (which documents the `pgtest` API and when real-DB tests are required).
- **Naming:** directories lowercase-hyphenated; Swift types PascalCase, no `I` prefix on protocols, classes `final` by default.

## Legal Documents

Privacy Policy and Terms live as JSON at `api-go/internal/legal/resources/{privacy,terms}.json`, embedded and served via `/v1/legal/{type}`, with a byte-equal iOS mirror. Use the `legal` skill; mechanically: edit the API JSON, run `scripts/sync-legal.sh`, commit both. CI fails on drift (`scripts/check-legal-drift.sh`).

## ADRs, Memos and Specs

- **ADR** (`docs/adr/NNNN-title.md`, copy `0000-template.md`: Status / Context / Decision / Consequences) for major architectural decisions: adopting or rejecting a technology, a structural pattern, a significant trade-off, or reversing a prior decision.
- **Memo** (`docs/memo/NNNN-title.md`, copy `0000-template.md`: Status / Question / Analysis / Options Considered / Recommendation) for analysis without a decision: trade-offs, future migration paths, options with no action taken. When a decision lands it graduates to an ADR (mark it `Superseded by ADR NNNN`).
- **Never commit spec files.** No `docs/specs/*.md`, no design markdown beside code, no per-feature plans; they rot faster than code and mislead. Beads track *what* and *dependencies*; the *how* and *why* live in the GitHub issue body ([Yegge, Issue #976](https://github.com/gastownhall/beads/issues/976)). Raise a self-contained issue with the `file-issue` skill (problem, approach, acceptance criteria, edge cases, test plan) and reference it from the bead (`GH: https://github.com/<org>/<repo>/issues/123`); workers read it with `gh issue view <n>`. If a bead is too thin, push the design into the issue.

## Beads

Use `bd` for ALL task tracking; never TodoWrite, TaskCreate or markdown files. `bd prime` loads the workflow context. Never use `bd edit` (interactive editor blocks agents).

**Never trust local Dolt state without syncing.** Many cloud agents and parallel local sessions read and write the same beads; the local replica goes stale within minutes with no warning (a bead can read `open` locally an hour after another session closed it and merged the PR).

- **Before any read you'll act or report on** (`bd show`, `bd list`, `bd ready`, epic status, choosing next work): `bd dolt pull`. A stale "still open" reads as confidently as a true one.
- **After every write** (`bd create`, `update`, `close`, `dep add`, `label`, …): `bd dolt push` immediately, not batched. An un-pushed write is invisible to other agents and can conflict or be lost. The bd pre-push hook (install once per clone: `bd hooks install`) runs `bd dolt push` on `git push`, but only covers writes that ride with a code push.

**Bead-first:** every code change needs a bead, even a one-line typo fix. `bd create --title="<change>" --type=task --priority=3`, `bd update <id> --claim`, `bd close <id>` when done. `.claude/require-bead.sh` blocks Write/Edit on code files with no in_progress bead; do not work around it.

- End every commit subject with `(<bead-id>)`, e.g. `fix: expire stale sessions (tc-a1b2)`, so `bd doctor` can detect orphans.
- File side-quests as new beads linked with `bd dep add <new> <current> --type=discovered-from`.

### Worktree-first

All local code changes happen in a worktree, never the main tree (parallel conversations conflict).

- **Location: `<repo>/.claude/worktrees/<name>`.** EnterWorktree auto-approves only there; elsewhere it prompts, which can't be pre-approved and stalls unattended sessions. So bd's default `<repo>/<name>` layout is banned: `bd worktree create .claude/worktrees/<name>` (path), but `bd worktree remove <name>` (bare name).
- **Use `scripts/wf/worktree-setup.sh <name> [--branch <branch>]`.** It resets local main to `origin/main`, runs `bd worktree create` (never raw `git worktree add`), verifies git registered the path, applies the bd workarounds (GH#3421 port symlink, beads#3593 chmod; remove when upstream fixes ship), resets the worktree to `origin/main`, and prints the absolute path. Its header documents the manual fallback. Then `EnterWorktree path: "<printed path>"`; finish with `/ship` or `ExitWorktree`.
- Hooks block Write/Edit on code files outside a worktree and raw `git worktree add`; do not work around them. Legacy worktrees at `<repo>/<name>` are still accepted so in-flight work isn't stranded; create new ones under `.claude/worktrees/`.
- **The orchestrator creates the worktree**, not the subagent, and dispatches workers with the path in hand, keeping create/verify/remove in one place.
- **Exception: cloud (web) sessions work in the main tree.** They run alone in a container on their own clone and branch, which already isolates them. Worktrees also fail there: bd's SEC-003 check reads home from the account database, not `$HOME`, and containers run as root (`/root`) with the repo under `/home/user`, so `bd worktree create` rejects `.beads` as an "unsafe location" (tc-aj2pn). `.claude/require-worktree.sh` exits early when `CLAUDE_CODE_REMOTE=true`.

### Dolt is the sole source of truth (DO NOT re-add issues.jsonl)

bd 1.0.4 server mode re-imported `.beads/issues.jsonl` on every command and could clobber fresh writes (gastownhall/beads #3849, fixed upstream 2026-05-26 via #4170). The workaround stays regardless: the jsonl is removed (archived at `~/.beads-archive/town-crier/`), `export.auto = false` and `export.git-add = false` in `.beads/config.yaml`, and only the pinned `~/.local/bin/bd` exists (never `brew install beads`: silent version drift). Sync is Dolt only, `bd dolt push`/`pull` against DoltHub `amyde/town-crier`; a fresh clone hydrates via `bd bootstrap`/`bd dolt pull`, never a jsonl import.

- **Never** re-create or re-track `.beads/issues.jsonl`, re-enable `export.auto`/`export.git-add`, or `bd export` to the default path.
- Re-running `bd init`, `bd init --server` or `bd setup claude`: always pass `--skip-agents`, or bd re-inserts a drifting duplicate of this section.
- Pinned to **stable bd 1.1.2** (from 1.0.4 on 2026-07-27, ADR 0046). Before bumping, check `gastownhall/beads#4800` and `#4176` (open server-mode schema-migration bugs matching our external dolt sql-server config).
- Root cause and recovery: `docs/memo/0011-beads-dolt-write-thrash-root-cause.md`, `docs/adr/0046-upgrade-bd-to-1.1.2.md`, auto-memory `project_bd_thrash_and_105_breakage`.

### Cleanup

Keep the working set (open + in-progress) under ~200.

- `bd flatten --force` squashes Dolt history when `bd` gets sluggish: the main speed lever, preferred over `bd compact`, whose squash can fail on a churned DB.
- `bd admin compact` (semantic decay of old closed issues), ~quarterly: `bd compact --analyze --json` → write summaries → `bd compact --apply --id <id> --summary -`.

## Shell & Tooling

- **Non-interactive flags always.** `cp`/`mv`/`rm` may be aliased to `-i`: use `cp -f`, `mv -f`, `rm -f`, `rm -rf`. Pass `--yes` or equivalent to fix/format CLIs.
- **RTK rewrites `rg`/`grep`** via a shell hook and can mangle output, making Grep come back empty. If a result looks wrong, use plain `grep`/`rg` in Bash or `rtk proxy <cmd>`.
- **Bash CWD persists between calls.** A stale `cd` makes a later reset/create hit the wrong tree; anchor with `git -C <repo-root> …` or absolute paths.
- **Single-quote bead notes.** A backtick in `bd update --notes "…"` triggers command substitution.

## Session Completion Checklist

When ending a session that changed code:

1. **File remaining work** as beads (`discovered-from` when it came out of this task).
2. **Write handoff notes** on the in-progress bead (beads survive compaction; conversation doesn't). Overwrite, don't append: `COMPLETED: … IN PROGRESS: … NEXT: … BLOCKER: … KEY DECISIONS: …`, so the next session resumes with zero context.
3. **Run quality gates:** tests, linters, builds.
4. **Update status:** `bd close` finished work, update in-progress items, `bd dolt push`.
5. **Sync and push:** on a feature branch `git pull --rebase && git push`; on main only `git fetch origin && git reset --hard origin/main`.
6. **Verify** `git status` shows "up to date with origin".

Proactively update auto-memory (`~/.claude/projects/.../memory/`) whenever you learn something noteworthy (architectural decisions or constraints, user preferences, workflow patterns, useful project context, corrections). Don't wait to be asked.
