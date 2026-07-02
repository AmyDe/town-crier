# 0013. Agentic workflow: token efficiency, determinism, and skill culling

Date: 2026-07-02

## Status

Open — most recommendations implemented 2026-07-02 (tc-m1xs); two high-blast-radius items deferred to dedicated beads.

### Implementation status (2026-07-02, tc-m1xs)

Landed in this change:

- **Skill cull:** deleted `autopilot`, `autopilot-loop`, `team-lead`, `triage-inbox`, `grill-me`, `test-consolidation-sweep`, `caveman`; removed stale `.claude/handoffs/`; removed the two `*-workspace/` eval-artifact dirs from `.claude/skills/`. 24 → 17 skills.
- **Worker model policy:** all six agent frontmatters set to `model: sonnet`; the double-pinned call-site `opus` is gone with the skills that carried it. Escalation rule ("re-dispatch once with Opus on gate failure", haiku for read-only fan-out, Sonnet goal-runner) documented in CLAUDE.md → "Worker model policy".
- **Prompt-tax:** the five slash-only maintenance skills (`adr-audit`, `sre-observatory`, `cost-forecast`, `legal`, `simplify-sweep`) marked `disable-model-invocation: true` — still work by slash, out of the auto-trigger catalogue.
- **Determinism:** added `scripts/wf/{worktree-setup,watch-pr,next-version,release-notes,close-released-issues}.sh`; wired `release` (version + notes + issue-close), `ship` (background gate watch, direct command kept as fallback), and `handover`/CLAUDE.md (worktree helper) to prefer them with the manual method retained.
- **Safety fixes:** `require-bead.sh` + `require-worktree.sh` now gate `*.go` and `.github/**/*.{yml,yaml}` (dropped dead `.cs`/`.csproj`); `pulumi-infra-worker` now loads `/go-coding-standards`; the duplicated worker-routing table was culled with the skills that held it, and `handover` now points at the single CLAUDE.md table; dangling references to deleted skills in `file-issue`/`legal`/CLAUDE.md fixed; `legal` now delegates copy voice to the `voice` skill.

Deferred (judgment-heavy, staged deliberately per the paying-customer stability posture):

- **Coding-standards core+references split** → bead `tc-a0ye`.
- **bd prime / `bd remember` memory pruning** → bead `tc-a5gt`.

## Question

The working loop (spec in a GitHub issue → `/handover` → `/goal` in a fresh session → TDD workers in worktrees → `/ship` → `/release`, plus `/loop`-driven maintenance skills) ships ~14 PRs and ~3 releases a day. But it feels token-heavy: subagents bias towards expensive models, and several LLM-narrated steps look scriptable. What should the full agentic workflow be, and which skills should be culled?

Hard constraint: subscription pricing. Nothing here may require headless `claude -p` (which forces API billing). Everything below runs inside interactive sessions, subagents of those sessions, or `/loop`.

## Analysis

Evidence base: all 24 skills + 6 agent definitions read in full; 332 session transcripts (2026-06-02 → 2026-07-01) mined for actual invocations; `gh` release/PR/issue history; git history of `.claude/`; the CI/scripts layer surveyed.

### What actually gets used (29 days of transcripts)

| Tier | Skill / command | Invocations | Note |
|---|---|---|---|
| Core | `/goal` | 75 | The single most-used thing in the system (harness built-in, not a skill) |
| Core | `ship` | 63 | Nearly always auto-triggered from natural language, never typed |
| Core | `release` | 61 | Same |
| Core | `escalation-protocol` | 27 | Workers escalating to phone via AskUserQuestion — working as designed |
| Core | `handover` | 24 | All within 3 days of its 06-28 rename; the current dispatch path |
| Active | `design-language` 18 · `voice` 16 · `file-issue` 14 · coding-standards triad (40/15/11 via workers) | | Keep |
| Rare | `ios-whats-new` 9 · `sre-observatory` 8 · `cost-forecast` 7 · `plan-to-beads` 6 · `simplify-sweep` 5 · `verify-polling` 4 · `adr-audit` 3 · `legal` 1 | | Keep, but stop paying for them in every session (below) |
| Never | `triage-inbox` 0 · `grill-me` 0 · `test-consolidation-sweep` 0 · `caveman` 0 · `team-lead` 0 · `autopilot-loop` 0 · `autopilot` 3 (all pre-switch) | | Cull |

Worker dispatches (~318 total): go-tdd-worker 104, Explore 42, general-purpose 40, ios-tdd-worker 35, pulumi-infra-worker 29, react-tdd-worker 21, github-actions-worker 18, delete-worker 17.

Explicit model overrides on those dispatches: **opus 117, sonnet ~96, haiku 0**. All six worker agents also pin `model: opus` in frontmatter, and `autopilot`/`team-lead` double-pin `"model": "opus"` at the call site. Every bead, including trivial CSS and YAML ones, pays Opus rates. Haiku has never been used for anything.

### Where the tokens actually go

1. **Opus on every worker dispatch.** ~117 Opus dispatches in 29 days, each an isolated context that re-reads the bead, the GitHub issue, the coding-standards skill, and iterates tests. This is the single biggest lever.
2. **Standards re-read per dispatch.** `go-coding-standards` is ~3,400 words and go-tdd-worker has a hard STOP instruction to load it before touching any `.go` file; 104 dispatches ≈ ~350k words of re-read reference material in a month, mostly rules the linters enforce mechanically anyway.
3. **System-prompt tax.** ~2,400 words of skill descriptions are injected into *every* session (332 sessions last month). The worst offenders are trigger-phrase lists on skills that are only ever invoked by typed slash command.
4. **`bd prime` at every SessionStart and PreCompact** injects ~12KB (and can fire twice per start). Ten persistent `bd remember` memories ride along every time.
5. **Live CI babysitting.** `ship` holds a full model turn through `gh pr checks --watch` (up to 10 minutes) and re-reads failed logs wholesale; 63 ships a month.
6. **The main session runs everything orchestral at the premium tier** (session default, currently Fable): `ship`, `release`, and `handover` are script-shaped work with only occasional judgment.
7. **The `/goal` brief is re-injected on every Stop-hook tick**, so brief verbosity is a recurring cost, not a one-off. (It is also silently truncated beyond ~3,000 chars.)

### Drift and foot-guns found on the way

- **`require-bead.sh` and `require-worktree.sh` do not gate `.go` files.** They only match `*.swift|*.cs|*.ts|*.tsx|*.css|*.csproj`. The Go API, CLI, and Pulumi infra — most of the codebase — can be edited on main with no bead and no worktree. The `.cs`/`.csproj` entries are dead post-.NET-migration.
- **`autopilot` and `autopilot-loop` were never actually removed.** Memory and the handover skill both say they were retired on 2026-06-26; git shows no deletion commit ever existed — the removal almost certainly happened in a working tree and was wiped by a `git reset --hard origin/main` (this repo's known failure mode). Both remain live and auto-triggerable: saying "ship the backlog" would invoke `autopilot-loop`, which arms a durable hourly cron that drains beads and **auto-releases** — directly against the post-paying-customer rollback posture.
- **The worker-routing/classification table exists in triplicate** (autopilot, team-lead, handover) plus CLAUDE.md, drifting independently. The post-merge test-command table is duplicated verbatim (autopilot, team-lead).
- `.claude/handoffs/` holds three stale April briefs that violate the current "never write handoffs into the repo" rule.
- `cost-forecast-workspace/` and `sre-observatory-workspace/` (~560KB of skill-creator eval artifacts) sit inside `.claude/skills/` where skill tooling scans.
- `legal` reimplements a subset of the `voice` rules inline instead of invoking the skill (drift risk).
- `pulumi-infra-worker` writes Go but never loads `go-coding-standards`.
- RTK's grep/log proxying has previously mangled output; `ship`'s CI-failure triage and lint parsing act on that output.

## Options Considered

**Worker model policy**

- **A. Opus everywhere (status quo).** Maximum one-shot quality; pays premium rates on every bead including one-line CSS fixes. No evidence Sonnet fails here, because it has never been tried on workers.
- **B. Static per-stack tiers.** Opus for go/pulumi (correctness-critical backend), Sonnet for react/ios/github-actions/delete. Simple, predictable, but still overpays on small Go beads and underserves an occasional hard web bead.
- **C. Sonnet-first, escalate on failure.** All workers default to Sonnet; the orchestrator re-dispatches the same bead with `model: opus` only when the worker fails its gates (tests/lint/build), stalls, or the post-merge/PR gate rejects the work. The existing safety nets (TDD red-green, lint hooks, pr-gate full suite, auto-merge only on green) convert "weaker model" into "occasional retry", not "broken main".

**Recommendation: C**, falling back to B if the retry rate proves annoying in practice. The retry loop is bounded and observable; the savings are on every dispatch.

**Where deterministic logic should live**

Plain scripts under `scripts/wf/` (called from skills via Bash), not the `tc` CLI (that is the product admin CLI; don't overload it) and not new LLM instructions. Scripts are version-controlled, testable, and cost zero tokens beyond their output.

## Recommendation

### The target workflow, end to end

1. **Spec** — unchanged in shape: discuss in the main session, then `file-issue` produces the self-contained GitHub issue. `plan-to-beads` for multi-slice epics. This is where the premium model genuinely earns its keep; keep your default model here.
2. **Handover** — `/handover` unchanged in role, slimmed in output: goal clause, issue number, worker route, acceptance checks. No spec restatement (the issue is the source of truth and the brief is re-injected every Stop tick). Reference CLAUDE.md's routing table instead of restating it.
3. **Goal-runner session** — start it with `/model sonnet`, then `/goal <brief>`. The goal-runner is a dispatcher and bookkeeper: it creates worktrees, launches workers, watches gates, merges. Sonnet handles that; save the premium model for sessions where you are actually designing. (Keep sending the `/goal` as its own message after `/clear` — the swallow race is real.)
4. **Worktree setup** — one script, `scripts/wf/worktree-setup.sh <name> [--branch <b>]`: reset local main to origin/main, `bd worktree create`, the GH#3421 port-file symlink, the #3593 `chmod 700`, print the path. Replaces the five-step recipe currently narrated from four different documents; when the upstream bd fixes ship, you change one file.
5. **Workers** — Sonnet-first per option C. Change all six agent frontmatters to `model: sonnet`; delete the call-site `"model": "opus"` double-pins so frontmatter is the single control. Orchestrator rule (one line in handover): "if a worker fails its pre-flight gates or the PR gate twice, re-dispatch the bead once with `model: opus`". Explore/locate dispatches get `model: haiku`; read-and-summarise dispatches get `sonnet`.
6. **Standards diet** — split each coding-standards skill into a ~800-word core (architecture rules, forbidden list, test-double conventions) plus reference files loaded on demand (`references/http-hardening.md` etc.). Workers must read the core; they pull references only when the bead touches that area. Roughly 2,500 fewer words per dispatch at current sizes.
7. **Ship** — flow unchanged (branch → PR → auto-merge.yml does the merging). Replace the in-turn `gh pr checks --watch` with `scripts/wf/watch-pr.sh <pr#>` run as a background Bash task that exits `MERGED` or `FAILED: <check list>`; the model re-engages once, and only reads targeted logs (`gh run view --log-failed` filtered to the failing job) on failure. Parse-critical output goes through `rtk proxy` to avoid mangling.
8. **Release** — `scripts/wf/next-version.sh` (semver bump from conventional commits since last tag) and `scripts/wf/release-notes.sh` (categorised changelog skeleton from commit subjects) do the mechanical 80%; the skill shrinks to: run scripts, sanity-check, `gh release create`, then `scripts/wf/close-released-issues.sh` for the bead→issue close sweep. LLM judgment remains only for ambiguous release-level calls and prose. (The public App Store copy stays fully LLM via `ios-whats-new` + `voice` — that is voice work, correctly so.)
9. **Maintenance skills** — keep `simplify-sweep`, `adr-audit`, `sre-observatory`, `cost-forecast`, `legal`, run ad hoc or via `/loop` when the backlog is drained (the "daily" framing in the sweep skills does not match reality and that is fine). Mark all five `disable-model-invocation: true`: their transcript evidence shows they are only ever invoked by typed slash command, so removing them from the model-visible catalogue deletes ~550 words of description from every session's system prompt and makes accidental auto-triggering impossible.
10. **Escalation / remote control** — unchanged. `escalation-protocol` → AskUserQuestion → phone notification is used (27 times) and works; do not touch it.
11. **bd prime diet** — prune the ten `bd remember` memories (at least the historical incident ones that have graduated into CLAUDE.md or memo 0011); every one rides into every session start and compaction.

### Skill culling

**Delete (7):**

| Skill | Why |
|---|---|
| `autopilot` | Superseded by handover→goal; 3 stale uses; classification table drift risk |
| `autopilot-loop` | Never invoked; arms a durable hourly auto-release cron that contradicts the paying-customer rollback posture; auto-triggers on "ship the backlog". Finish the deletion that 06-26 intended |
| `team-lead` | Never used; near-duplicate of autopilot's dispatch/merge logic; Agent-Teams parallelism unneeded at a 14-PR/day single-operator cadence |
| `triage-inbox` | Zero uses in a month; issue flow goes through file-issue/plan-to-beads now. If phone-filed issues return, resurrect from git history |
| `grill-me` | Zero uses; if you miss the interview style, add one optional line to file-issue ("interview me one question at a time first") |
| `test-consolidation-sweep` | Zero uses; fold its mutation-gate idea into simplify-sweep's brief as a category if ever wanted |
| `caveman` | Zero uses; harness compaction and concise-mode do this better |

Also delete `.claude/handoffs/` (three stale April briefs) and move `cost-forecast-workspace/` + `sre-observatory-workspace/` out of `.claude/skills/` (archive under `docs/` or delete; they are eval artifacts, not skills).

**Keep, modified:** the five maintenance skills (add `disable-model-invocation: true`), the three coding-standards skills (core+references split), `handover` (slimmed, reference the CLAUDE.md routing table), `legal` (delegate to `voice` instead of inlining its rules), `release`/`ship` (thin wrappers over the new scripts), `verify-polling` (optionally script the KQL battery, low priority at 4 uses/month).

**Keep as-is:** `file-issue`, `plan-to-beads`, `escalation-protocol`, `design-language`, `voice`, `ios-whats-new`.

Net effect on the every-session prompt: 7 deletions + 5 `disable-model-invocation` + trimmed descriptions ≈ half the current ~2,400-word skill-catalogue tax gone, and the dangerous auto-triggers with it.

### Safety fixes (do these regardless of everything else)

1. **Hook coverage:** add `*.go` (and `.github/**/*.yml`) to the `require-bead.sh` / `require-worktree.sh` matchers; remove the dead `.cs|.csproj` entries. Today the bead-ledger and worktree-isolation guarantees do not apply to most of the codebase.
2. **One routing table:** CLAUDE.md's stack table is canonical; handover points at it. Delete the copies as part of the culls above.
3. `pulumi-infra-worker`: mandate the go-coding-standards core in Setup.
4. Correct the stale memory/doc claims that autopilot was already removed (true once the cull lands).

### Rough sizing of the win

- **Worker model shift is most of it:** Sonnet in place of Opus on the ~three-quarters of dispatches that never needed Opus, with bounded escalation. Subscription usage limits track model cost, so this directly extends how much work fits in a week.
- Standards diet: ~2,500 words less per worker dispatch, ~10 dispatches/day.
- Prompt tax: ~1,200 words less per session, hundreds of sessions/month; plus smaller `bd prime`.
- CI watching: one held premium-model turn per ship (63/month) becomes approximately zero.
- Nothing user-facing changes: pr-gate, auto-merge, TDD protocol, release trains, escalation-to-phone all stay exactly as they are.

### Considered and rejected

- **Headless/cron Claude for drains and sweeps** — vetoed up front (API billing). `/loop` in a local session remains the only scheduler.
- **Agent-Teams parallel bead drains (`team-lead`)** — coordination overhead without a throughput problem to solve.
- **Scripting the spec/design stages** — `file-issue`, `grill-me`-style interviewing, ADR writing, and store copy are judgment work; making them deterministic would just make them worse. The deterministic layer (auto-merge.yml, ios-release-notes.sh, legal drift check, seo-refresh, ACA cleanup, `tc` CLI) already covers what should never have been LLM work, which is why this memo adds scripts rather than workflows.
