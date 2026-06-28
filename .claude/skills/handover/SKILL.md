---
name: handover
description: "Produce a self-contained handover brief for a fresh, goal-driven session. The brief is written to be pasted into a new session with the `/goal` command: it states the goal clause in plain language, anchors it to a GitHub issue spec (the source of truth), and tells the new session to deliver via the matching TDD worker agent — or a raw subagent when the work sits outside the TDD worker boundaries. MUST use whenever the user asks to 'hand off', 'handover', 'write a handoff', 'produce a handoff', 'hand this off to a fresh session', 'handoff for /goal', 'goal handoff', or invokes '/handover'. This is the Town Crier handover (distinct from Gas Town's `/handoff` / `gt handoff`, which is not used here)."
---

# Handover

Produce a **handover brief** that hands the current work to a fresh Claude session driven by the `/goal` command. The brief is the text the user pastes after `/goal` — so it must be self-contained, name a single testable goal, anchor that goal to a GitHub issue spec, and tell the new session exactly how to deliver.

This replaces the old `autopilot` / `autopilot-loop` skills. There is no autonomous drain loop any more: the user opens a fresh session, runs `/goal`, pastes this brief, and the goal-armed session works the issue to completion using the project's TDD workers.

> `/goal` arms a session-scoped Stop hook that keeps the session working until the goal is met. The whole brief becomes the goal description, so write it as one — a clear objective, a "done when", and everything the new session needs to act without conversation history.

> ⛔ **HARD LIMIT: the emitted brief MUST BE UNDER 3000 CHARACTERS.** `/goal` truncates longer input, so an over-length brief silently loses its tail (guardrails, next step). This is non-negotiable: condense ruthlessly, fold multi-slice scope into terse clauses, drop nice-to-haves before essentials, and **measure the character count before emitting** — never eyeball it. Under 3k or it does not ship.

## What you are NOT doing

You are writing a brief, not implementing. Do not start coding, do not dispatch workers, do not create worktrees. Gather state and emit the brief.

## Phase 1: Gather state

Pull together everything a context-free session needs. Use what's already in this conversation first; fill gaps with:

- **The bead(s):** `bd show <id>` for the in-progress / target bead — title, acceptance, notes (`COMPLETED / IN PROGRESS / NEXT / BLOCKER / KEY DECISIONS`), and any `GH:` issue reference.
- **The spec issue:** the GitHub issue is the source of truth (this project never keeps spec files in the repo). Find it from the bead's `GH:` line or description. Read it: `gh issue view <n>`. If there is **no** issue yet, stop and tell the user to raise one first with the `file-issue` skill — the handover must reference a spec, not invent one.
- **Git state:** current branch, and what's already landed: `git log main..HEAD --oneline` (or the relevant base).
- **Decisions and blockers:** decisions already made (so the next session doesn't relitigate them), and anything blocking.

## Phase 2: Decide delivery routing

Work out how the new session should deliver, and bake the answer into the brief.

**Matching TDD worker** — route by where the code lives:

| Work area | Worker agent | Allowed path |
|-----------|--------------|--------------|
| iOS / Swift | `ios-tdd-worker` | `mobile/ios/` |
| Go API / CLI | `go-tdd-worker` | `api-go/`, `cli/` |
| React / TypeScript / web | `react-tdd-worker` | `web/` |
| Pulumi infra (Go) | `pulumi-infra-worker` | `infra/` |
| GitHub Actions / CI | `github-actions-worker` | `.github/` |
| Delete/remove + tech area | `delete-worker` | *(by tech area)* |

UI work in any stack also consults the `design-language` skill alongside the platform skill.

**Out-of-boundary fallback** — if the work does **not** fit any TDD worker (docs, ADRs/memos under `/docs`, root-level scripts, cross-cutting tooling, `.claude/` config, anything with no `*-tdd-worker` home), say so in the brief and instruct the new session to dispatch a **raw `general-purpose` subagent** with a tailored prompt: name the exact files, the acceptance, and the same guardrails (bead-first, worktree, green tests/build). Never force a misfit worker onto out-of-boundary work.

If the scope spans more than one worker area, list each slice and its worker, and note the producer→consumer order (e.g. API before its iOS/web clients).

### Visual verification routing (iOS or web UI changes)

If the handover changes anything an **iOS or web user sees or does**, the brief MUST require the new session to **visually verify the change locally before it ships** — build/run the app, drive it, and confirm the UI is right *before* the PR opens. A green test suite does not prove a layout, a map, or a flow looks correct. Skip this only for backend/infra/CI/docs handovers with no user-visible surface.

Two non-negotiables, baked into the brief:

- **Always drive the app from a Sonnet subagent — never the orchestrator.** mobile-mcp and agent-browser emit token-heavy screenshots that bloat the main context fast. The orchestrator dispatches a `general-purpose` subagent with `model: sonnet` that navigates, screenshots, and reports findings back as **text** (pass/fail + what it saw), never raw images into the main loop.
- **Pick the tool by stack.** iOS → **mobile-mcp** (project MCP; golden `xcodegen → clean build → boot sim → install → drive` path in memory `reference_ios_simulator_build_deploy`). Web → **agent-browser** (Homebrew CLI on `PATH`: `open`/`snapshot`/`click`/`screenshot`; screenshot paths MUST be absolute — memory `reference_agent_browser_cli`).

**Simulating data-dependent scenarios entirely locally (no remote infra).** When the change only shows up with data (a populated list, a clustered map, a tier-gated state) **and** needs a signed-in user, the deployed dev API **won't have your seed** — so stand the *whole* stack up on the workstation. The complete, load-bearing recipe (with the non-obvious bits) lives in memories `reference_local_api_stack_and_seed` + `reference_local_web_browser_verification_auth`; the parts the next session keeps tripping on:

- **Postgres:** `make -C api-go db-up` boots PostGIS on `:5433` but does **NOT** migrate. The goose **CLI** fails here (pulls missing drivers), and `cmd/pgmigrate` is Azure-only — so apply migrations via a tiny throwaway module that calls goose as a **library** with only `lib/pq`, pointed at `internal/platform/postgres/migrations`. Then **seed** with raw SQL: tiers are capitalized (`Free`/`Personal`/`Pro`), apps match zones **geographically** (`ST_DWithin`, so just place them within `radius_metres` of the zone centre), and "unread" = a `notifications` row whose `created_at` is newer than `notification_state.last_read_at`.
- **API:** `TEST_DATABASE_URL=postgres://towncrier:towncrier@localhost:5433/towncrier_test?sslmode=disable`, **`POSTGRES_AUTH` UNSET** (selects the password pool), plus `AUTH0_DOMAIN=towncrierapp.uk.auth0.com AUTH0_AUDIENCE=https://api-dev.towncrierapp.uk CORS_ALLOWED_ORIGINS=http://localhost:5173 PORT=8080`, then `go run ./cmd/api`. It validates real Auth0 JWTs against the live JWKS, so an injected dev token works.
- **Web auth wall (the big one):** the dev Auth0 SPA does **NOT** whitelist `http://localhost` callbacks, so a redirect login is impossible (403). `cd web && npm run dev -- --port 5173 --strictPort` with `VITE_API_BASE_URL=http://localhost:8080` + the dev `VITE_AUTH0_*`, then mint a token with the Auth0 **password grant** and inject it into `auth0-spa-js`'s localStorage cache (`@@auth0spajs@@::<clientId>::<audience>::<scope>`) so `getTokenSilently` serves it with no redirect. Dev SPA `client_id=rgP7yhxRKByriQGa7RHElaGVmXslbVjV`, `audience=https://api-dev.towncrierapp.uk`. Reusable inject/seed scripts + the exact recipe are in the web memory.
- **iOS** DEBUG hardcodes `api-dev.towncrierapp.uk` (`APIEnvironment.swift`), so iOS verifies against **deployed dev** + the dev login by default; only seed locally if you also repoint the build at localhost.

**Dev test login (non-sensitive — keep it in this skill and in the brief):** `christy+tctest10@salter.uk` / `StrongPassword1!`. Use it for the password grant above (local authed+seeded web) or to sign in directly against deployed **dev** (iOS DEBUG, or web pointed at the dev API) when no local seed is needed.

### Execution-environment gotchas (carry a terse line into the brief)

These each cost a real session a cycle — bake them in so the next one doesn't rediscover them:

- **RTK mangles shell output.** A shell hook proxies `curl`, `eslint`, `vitest`, `grep`/`rg` through RTK, which can dump output to stdout (it once **leaked an access token** and broke a pipe), invent phantom lint errors, or print a false "SCOPE VIOLATION". For anything whose output you parse, bypass it: `/usr/bin/curl`, `node ./node_modules/eslint/bin/eslint.js src`, `node ./node_modules/vitest/vitest.mjs run`, plain `grep`, or `rtk proxy <cmd>`.
- **`curl --data-urlencode` for any value with `+`/reserved chars** — the `+` in the test email sent via plain `--data` is form-decoded to a space → `invalid_grant` "wrong email or password".
- **`bd worktree create` bases the new branch off the orchestrator tree's CURRENT HEAD, not `origin/main`.** On any branch other than an up-to-date `main` the worktree is wrong-based (missing already-merged work). Right after creating it: `git -C <wt> reset --hard origin/main` and confirm `git -C <wt> log -1` == `origin/main` before dispatching the worker.
- **Verify each bead against its OWN worktree's build.** Two beads in flight = two worktrees, but only one Vite/sim at a time — don't verify bead B against bead A's running build (it silently shows the wrong result; swap the dev server to the right worktree first).

Tool limits to flag rather than paper over: mobile-mcp has no pinch/two-finger gesture (map zoom-*out* can't be driven) and a still screenshot can't show jank/frame-rate — for those, the subagent reports "needs a human eye", it does not claim verified.

## Phase 3: Emit the brief

Output the brief to the user as a single fenced block they can copy. Tell them plainly: **run `/goal` in a fresh session and paste this as the goal.** Fill every placeholder from Phase 1–2; drop any line that genuinely doesn't apply rather than leaving a placeholder.

**MUST verify length before you emit.** Write the brief to its temp deliverable file (see Phase 4 — outside the repo) and measure it — e.g. `python3 -c "print(len(open('<file>').read()))"` (do not trust `wc -c`; a shell proxy can mangle it). If the count is **≥ 3000**, condense and re-measure until it is **< 3000**; only then print the fenced block. State the final character count in one line beneath the block so the user can see it cleared the limit. Never emit a brief you have not measured.

````
GOAL: <one-sentence objective>. Keep working until this is delivered and shipped.
DONE WHEN: <testable acceptance — what must be true to call it finished>.

SPEC (source of truth): <github-issue-url>
Read it first: `gh issue view <n>`. The issue body holds the what/how/why and the
acceptance criteria, and is authoritative — if it conflicts with this brief, the issue
wins and you flag the discrepancy rather than guessing.

BEAD(S): <tc-id> — <title>. Claim before editing: `bd update <tc-id> --status=in_progress`.
Close it when the acceptance is met. (No bead yet for some slice? Create one — every code
change needs a bead.)

DELIVERY — you are the orchestrator; you do NOT write code yourself:
  1. Worktree first. Create an isolated worktree (`bd worktree create <name> --branch <branch>`),
     apply the two bd worktree workarounds (GH#3421 port symlink + `chmod 700 .beads/`, see
     CLAUDE.md), then `git -C <wt> reset --hard origin/main` (it bases off the current branch's
     HEAD, NOT origin/main — verify `log -1` matches). Dispatch the worker with the path in hand.
  2. Dispatch the matching TDD worker for the tech area:
       <selected worker(s) and allowed path(s) from Phase 2>
     The worker consults its coding-standards skill; UI work also consults design-language.
  3. OUT OF BOUNDARY: <only if applicable> this work has no TDD worker home, so dispatch a
     raw `general-purpose` subagent with a prompt that names the exact files (<files>), the
     acceptance, and the guardrails below. Do not force a misfit TDD worker onto it.
  4. Validate: run the stack's test/build command and confirm it passes (`go test ./...` /
     `swift test` / `npx vitest run`). UI change? ALSO verify locally (see VERIFY LOCALLY) —
     do NOT ship until the running app shows the change correct. Then ship via `/ship` — PR +
     gate, never a direct push to main.

CURRENT STATE:
  - Branch: <branch>
  - Landed: <what's already done / merged>
  - Remaining: <the concrete slices left, in order>

VERIFY LOCALLY <keep only for an iOS/web UI change; delete for backend/infra>: before the PR,
drive the running app from a SONNET subagent (agent-browser=web, mobile-mcp=iOS; reports TEXT,
screenshots bloat context) and eyeball the change. Data-dependent + authed web ⇒ run the FULL
stack locally (deployed dev has no seed): `make -C api-go db-up` then migrate (goose-as-library,
lib/pq) + seed, local `cmd/api` (POSTGRES_AUTH unset, CORS localhost:5173), Vite :5173, and an
Auth0 password-grant token injected into localStorage (dev SPA blocks localhost login). Recipe +
scripts: memories reference_local_api_stack_and_seed + reference_local_web_browser_verification_auth.
Test login: christy+tctest10@salter.uk / StrongPassword1! (non-sensitive). Verify each bead on
its OWN worktree build. ENV: RTK mangles curl/eslint/vitest/grep — use /usr/bin/curl, the
node_modules bins, plain grep; `curl --data-urlencode` for the `+` in the email.

KEY DECISIONS (already settled — don't reopen): <decisions, or "none">
BLOCKERS: <blocker, or "none">
NEXT CONCRETE STEP: <the single first action the new session should take>

GUARDRAILS: worktree-first; bead-first (a bead per code change, closed when done); tests/build
green before ship; PR-only — no direct deploys via az/pulumi/git push to main; fix-forward
(pre-revenue, no rollback/soak). Hold and flag anything privacy/GDPR/auth/telemetry-sensitive
that the issue doesn't explicitly authorise rather than committing it.
````

After the block, in one or two lines, point the user at the spec issue URL and the bead id so they can sanity-check before pasting.

## Phase 4: Save outside the repo, copy to clipboard, print a clickable link

Always (not optional) persist the brief to the user's **temp directory — NEVER inside the repo** (it is not a spec; in-repo handoff files rot like spec files do), put it on the clipboard, and print a cmd-clickable link. This is the same file you measured in Phase 3.

```bash
dir="${TMPDIR:-/tmp}"; dir="${dir%/}"                 # user's macOS temp; strip trailing slash
dest="$dir/town-crier-handover-<short-slug>.md"        # descriptive, stable name
# write the brief straight to "$dest" (this is the file you measure in Phase 3)
pbcopy < "$dest"                                        # macOS: brief now on the clipboard
printf 'file://%s\n' "$dest"                            # cmd-clickable link in the terminal
```

Then tell the user plainly, in one or two lines: the brief is **on your clipboard** (paste straight after `/goal`), saved at the **cmd-clickable `file://…` link**, and N characters (under the 3k cap). Still print the fenced block in chat as a preview. **Never** write the handoff into `.claude/` or anywhere under the repo — only the temp dir.

Also remind the user how to confirm it armed: send `/clear` and `/goal` as **separate** messages (never one burst — the `/clear`→`/goal` race silently swallows the command), then check the session prints **`Stop hook is now active`** and actually starts working. If you see only `Goal set:` then silence, the `/goal` was swallowed — re-send it on its own. A fresh session can't install that Stop hook from inside itself, so re-sending `/goal` is the only fix. See memory `feedback_goal_clear_race`.

## Rules

- **Under 3000 characters, measured — not estimated.** The brief is hard-capped at <3k chars because `/goal` truncates beyond it. Write it to a file, count it (`python3 -c "print(len(open('f').read()))"`), and condense until it clears. An over-length brief is a defective brief.
- **Reference a real GitHub issue.** The spec is the issue, never a repo file. No issue → tell the user to raise one with `file-issue` first.
- **One goal, testable.** The brief names a single objective with a concrete "done when". If the scope is really two goals, say so and produce two briefs.
- **Self-contained.** Assume zero conversation history. Anything the new session needs goes in the brief.
- **TDD worker by default, raw subagent only out of boundary.** Pick the worker from the table; reach for a `general-purpose` subagent only when nothing fits, and give it a tailored prompt.
- **Orchestrator never writes code.** The brief always routes implementation through a worker or subagent in a worktree, validated by tests, shipped via PR.
- **Visual-verify iOS/web UI changes before ship — always via a Sonnet subagent.** Any user-visible iOS or web change in the handover must instruct the new session to drive the running app and eyeball the change locally *before* the PR opens, and that driving (mobile-mcp / agent-browser) must run in a `model: sonnet` subagent that reports findings as text — their screenshots bloat context. Use the local Docker stack (`make -C api-go db-up` + local `cmd/api`) to simulate data-dependent scenarios without remote infra. Omit the VERIFY LOCALLY block entirely for backend/infra/CI/docs handovers.
- **Carry the execution-environment gotchas.** Any brief that involves a worktree, shell tooling, or local web/iOS verification must surface the gotchas from Phase 2 (RTK mangling → bypass binaries; `bd worktree create` wrong-bases → reset to origin/main; `--data-urlencode` for the `+` email; the localhost auth wall + password-grant injection; verify each bead on its own build). Inline the test creds; lean on the two local-stack memories for the long-form recipe so the brief stays under 3k.
- **Don't implement.** This skill produces the brief and stops.
