---
name: ship
description: Automate the push-to-PR flow when you have local commits and/or unstaged changes on main, or when you're on a feature branch (e.g., from a worktree) ready to PR. Creates a feature branch (if on main), opens a PR via `gh`, and watches for the PR Gate CI check to pass. Stops there by default and reports status — the repo's Auto-merge workflow is intentionally disabled so Claude PR-triage routines can review comments (e.g. CodeRabbit) before merge, so this skill does not merge the PR itself unless the user explicitly asks for it in a live session. MUST use this skill whenever the user says "ship", "ship it", "push to main", "push my changes", "get this on main", "create a PR", or any variation of wanting to get local work through CI and up for review. Also trigger when the user has been working on main and wants to push but can't due to branch protection, or when work on a feature branch/worktree is ready to ship. Do NOT use for: merging a PR on request (see Step 7a) or for automated/scheduled contexts wanting a merge (never merge in those — see CLAUDE.md).
---

# Ship to Main

Route local work on `main` through a PR, because direct pushes to main are blocked by branch protection and a local pre-push hook. This skill handles branch, push, create PR, and wait for CI — then **stops**. Merging is a separate, explicit step (Step 7a), never automatic.

## How merging works

There are no reviewer approvals required by GitHub itself — CI is the sole required status check. But this repo does not auto-merge on green CI:

1. **`pr-gate.yml`** — The CI pipeline. It detects which areas changed (API, iOS, web, infra) and runs only the relevant checks. A single **`gate`** job aggregates all results — this is the sole required status check for branch protection.
2. **`auto-merge.yml` is intentionally disabled.** Claude PR-triage routines watch open PRs and handle merging themselves once they've reviewed CodeRabbit and other review comments — see CLAUDE.md, "PR merge — no auto-merge, Claude-routine triage". A green `gate` means **ready for triage**, not merged.
3. This skill's default stopping point is therefore "PR Gate passed, PR open" — not "merged." Only merge yourself (Step 7a) when the user is live in the session and explicitly asks you to; never in an automated or scheduled context (cron routine, unattended `/loop`).

## Workflow

### Step 1: Pre-flight

1. Run `git fetch origin` to get the latest remote state.
2. Detect which branch you're on:
   - **On `main`:** proceed normally through Steps 2–5.
   - **On a feature branch** (e.g., work done in a worktree): skip Steps 2–4 and go directly to Step 5 (Push and create PR). Note the branch name and whether you're in a worktree (`git worktree list`) for cleanup later.
   - **On an unrelated branch with uncommitted work:** stop and tell the user.
3. Check there's actually work to ship:
   - `git log origin/main..HEAD --oneline` — local commits ahead of origin
   - `git status --short` — unstaged or untracked changes
   - If both are empty, tell the user there's nothing to ship and stop.

### Step 2: Handle unstaged/untracked changes

If `git status --short` shows changes:

1. Review what's there and decide what to include.
2. **Dotfile/dot-directory judgement:** The goal is to include intentional project files and exclude ephemeral tool artifacts. Use these heuristics:
   - **Include:** `.editorconfig`, `.swiftlint.yml`, `.gitignore`, `.github/` workflows, config files clearly part of the project. If similar files in that directory are already tracked (`git ls-files`), include the changes.
   - **Exclude:** `*-workspace/` directories (skill-creator artifacts), `.DS_Store`, editor swap files (`.*.swp`, `*~`), evaluation outputs, temporary tool state. These are transient artifacts, not project files.
   - When genuinely uncertain, lean toward including — the user can always revert, but missing files require re-doing work.
3. Stage files with `git add <specific files>` — never `git add .` or `git add -A`, since that bypasses the filtering.
4. Commit with a concise, descriptive message summarizing the unstaged changes.

### Step 3: Derive branch name

Look at all commits going into the PR (`git log origin/main..HEAD --oneline`):

- **Single commit:** slugify its message → `auto/add-user-authentication`
- **Multiple commits:** find the dominant theme or use the most significant change → `auto/add-react-tdd-worker`
- Format: `auto/<slug>` — lowercase, hyphens, max ~50 chars, no special characters.

### Step 4: Create branch and reset main

```bash
git checkout -b <branch-name>
git branch -f main origin/main
```

This is the key move. The feature branch now holds all work, and local `main` is reset to match origin. Both operations are safe because we've already verified everything is committed.

### Step 5: Push and create PR

```bash
git push -u origin <branch-name>
```

Then create the PR:

```bash
gh pr create --title "<concise title under 70 chars>" --body "$(cat <<'EOF'
## Changes
<bullet list summarizing each commit>

---
*Auto-shipped via ship skill*
EOF
)"
```

**PR title:** Summarize all changes in one line. If there's a single commit, use its message. If multiple, synthesize a summary.

**PR body:** List each commit as a bullet point under "## Changes".

**Default behavior:** Report the PR URL and proceed straight to Step 6 (watch CI). Do not ask — just do it. The user expects to walk away after typing "ship it", to a PR that's through CI and waiting for triage, not necessarily merged. Do not run `gh pr merge` at this point — that's Step 7a, and only on explicit request.

> PR created: <url>
>
> Watching for PR Gate...

**Exception:** If the user explicitly says "leave it open", "don't merge", or similar — report the PR URL and stop. The skill ends here.

### Step 6: Watch for CI checks

The **PR Gate** (`gate` job in `pr-gate.yml`) is the sole required status check. It runs checks only for changed areas:

| Area | Checks | Trigger |
|------|--------|---------|
| API | Format, Build & test, Staging deploy, Integration tests, Staging cleanup | Files in `api/` changed |
| iOS | SwiftLint, Build & test | Files in `mobile/ios/` changed |
| Web | Lint, Type-check + test + build | Files in `web/` changed |
| Infra | Pulumi preview | Files in `infra/` changed |

The `gate` job passes if every triggered check passes (skipped checks are fine).

**Watch for checks to complete.** Preferred: run `scripts/wf/watch-pr.sh <pr-number>` as a **background** Bash task — it blocks on the gate and prints one verdict (`GATE_PASSED` / `FAILED: <checks>` / `TIMEOUT`), so you don't hold a live model turn babysitting the watch; re-engage only on the result and diagnose only on `FAILED`. It only watches the gate — it never polls for or performs a merge, since auto-merge is disabled. It wraps the same underlying command, which stays the direct fallback:

```bash
gh pr checks <pr-number> --watch --fail-fast
```

Run this with the Bash tool's **timeout set to 600000ms** (10 minutes). The command blocks until all checks complete (exit 0 = all passed) or any check fails (non-zero exit).

- **Exit 0** → all checks passed. Proceed to Step 7.
- **Non-zero exit** → a check failed. Enter the **failure handling** below.
- **Timeout** → report the PR URL and stop. Nothing is lost — the PR stays open, checks keep running server-side, and you (or a later invocation) can re-check with `gh pr checks <pr-number>`.

If you need to inspect individual check statuses (e.g., after a failure), query with JSON:

```bash
gh pr checks <pr-number> --json name,state,bucket
```

**IMPORTANT:** The field is `bucket` (not `conclusion`). Valid bucket values: `pass`, `fail`, `pending`, `skipping`.

### Step 6a: Handle CI failures

If any check fails:

1. **Identify what failed.** Fetch the failed check details:
   ```bash
   gh pr checks <pr-number>
   gh run view <run-id> --log-failed
   ```

2. **Assess each failure:**
   - **Format/lint failure** (e.g., `gofmt`, `swiftlint`, `npm run lint`): Fix it locally, commit, and push. These are mechanical fixes.
   - **Test failure:** Read the test output, diagnose the issue, fix it, commit, and push.
   - **Build failure:** Read the build output, fix the issue, commit, and push.
   - **Infrastructure/deployment failure** (staging deploy, integration tests): These may involve secrets or environment issues. Report the failure details and the PR URL, then stop — the user needs to investigate.

3. **For fixable failures:** Make the change, commit, and push:
   ```bash
   git add <files>
   git commit -m "fix: <brief description of what was fixed>"
   git push
   ```
   This re-triggers the CI pipeline. Return to the poll loop in Step 6.

4. **Limit:** Make at most **3 rounds** of fixes. If the gate still hasn't passed after 3 rounds, report the outstanding failures and the PR URL, then stop. The PR stays open either way.

### Step 7: Report and stop

Once `gate` passes, **stop here by default.** Report:

> PR Gate passed: <url>
>
> Ready for triage — not merged. This repo's Auto-merge is intentionally disabled so Claude PR-triage routines can review CodeRabbit and other comments before merge (CLAUDE.md, "PR merge — no auto-merge, Claude-routine triage"). Say so if you'd like me to merge it now.

Do not run `gh pr merge`, do not clean up the worktree/branch, and do not touch beads sync here — none of that is appropriate until the PR is actually merged, and merging isn't this skill's default job anymore. The skill ends here unless Step 7a applies.

### Step 7a: Merge on explicit request (interactive sessions only)

Only do this when the user is live in the session and explicitly asks for the PR to be merged now (e.g. "merge it", "go ahead and merge #1028"). **Never do this in an automated or scheduled context** — a cron routine, an unattended `/loop`, or any invocation not driven by the user typing in real time. Work completed on a schedule can land while the user has no chance to look at it first; that's exactly the case the disabled Auto-merge workflow and the triage routines exist to cover. If you're unsure whether the current invocation counts as "live," don't merge — report gate-passed (Step 7) and ask.

```bash
gh pr merge <pr-number> --squash
```

Then clean up:

```bash
gh pr view <pr-number> --json state -q '.state'   # confirm MERGED
bd worktree list
```

**If you're in a worktree** (feature branch is in a secondary worktree, `main` is in the primary):

1. Note the primary worktree path (the one on `main`).
2. Remove the worktree and delete the branch:
   ```bash
   bd worktree remove <this-worktree-name> --force
   git -C <primary-path> branch -D <branch-name>
   ```

**If you're on the feature branch directly** (no worktree):

1. Switch to main and delete the branch:
   ```bash
   git checkout main
   git branch -D <branch-name>
   ```

**Then, from the primary worktree (on `main`), reset and prune:**

1. Stash any uncommitted changes first (other in-progress work on main):
   ```bash
   git status --short
   # Only if there are changes:
   git stash -u
   ```
2. Hard-reset to origin/main. Squash-merge means any local commits on main are already represented in the remote squash commit — rebasing them creates guaranteed conflicts on files like `.beads/issues.jsonl` that both branches touched. Reset is the correct move:
   ```bash
   git fetch origin
   git reset --hard origin/main
   ```
3. Restore stashed changes:
   ```bash
   git stash pop   # only if you stashed above
   ```
4. Prune stale remote-tracking refs. **Do NOT use `git push origin --delete`** — the pre-push hook blocks it on main, and GitHub auto-deletes merged branches anyway:
   ```bash
   git remote prune origin
   ```
5. Verify only `main` remains locally:
   ```bash
   git branch -a
   ```

### Step 8: Sync beads and verify (after Step 7a only)

Push beads data to the Dolt remote — this is separate from git and must happen after every merge:

```bash
bd dolt push
```

Then run `git status` and `git log --oneline -3` to confirm:
- On `main`, up to date with origin
- The squash-merged commit is visible

Report success with the PR URL.

## Error Handling

- **Not on main or a feature branch:** Stop. Tell the user which branch they're on and ask what to do.
- **Nothing to ship:** Tell the user. Don't create empty PRs.
- **CI gate failed:** Report which checks failed, show relevant log output, and the PR URL. Fix mechanical issues (format, lint, tests) up to 3 rounds. For infrastructure/environment failures, stop and report.
- **Timeout (15 min, gate not complete):** Report current check states and PR URL. The PR stays open — no work lost.
- **gh CLI not authenticated:** Tell the user to run `gh auth login`.
