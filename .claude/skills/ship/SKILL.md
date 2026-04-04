---
name: ship
description: Automate the push-to-main flow when you have local commits and/or unstaged changes on main. Creates a feature branch, opens a PR via `gh`, then watches for the PR Gate CI check to pass — auto-merge is enabled automatically by a GitHub Actions workflow. MUST use this skill whenever the user says "ship", "ship it", "push to main", "push my changes", "get this on main", "merge to main", "create a PR and merge", or any variation of wanting to get local work from main onto the remote. Also trigger when the user has been working on main and wants to push but can't due to branch protection. Do NOT use for: creating PRs without merging, pushing feature branches, or work that isn't on main.
---

# Ship to Main

Route local work on `main` through a PR, because direct pushes to main are blocked by branch protection and a local pre-push hook. This skill handles the flow: branch, push, create PR, wait for CI, and merge — fully hands-off by default.

## How merging works

There are no reviewer approvals required. The merge flow is entirely CI-driven:

1. **`auto-merge.yml`** — A GitHub Actions workflow that automatically enables squash auto-merge on every newly opened PR. The skill does NOT need to run `gh pr merge --auto`.
2. **`pr-gate.yml`** — The CI pipeline. It detects which areas changed (API, iOS, web, infra) and runs only the relevant checks. A single **`gate`** job aggregates all results — this is the sole required status check for branch protection.
3. When `gate` passes, auto-merge fires and the PR is squash-merged automatically.

## Workflow

### Step 1: Pre-flight

1. Confirm you're on `main`. If not, stop and tell the user — don't switch branches when there might be in-progress work elsewhere.
2. `git fetch origin` to get the latest remote state.
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

**Default behavior:** Report the PR URL and proceed straight to Step 6 (watch CI). Do not ask — just do it. The user expects to walk away after typing "ship it". Auto-merge is enabled automatically by the `auto-merge.yml` workflow — do not run `gh pr merge --auto` yourself.

> PR created: <url>
>
> Auto-merge will be enabled by CI. Watching for PR Gate...

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

**Poll loop** (repeat every 30 seconds, up to 15 minutes):

```bash
gh pr checks <pr-number> --json name,state,conclusion
```

**On each poll iteration:**

1. Check if the `PR Gate` check has completed:
   - **Conclusion `success`** → proceed to Step 7.
   - **Conclusion `failure`** → enter the **failure handling** below.
2. If still pending, report which checks are running and continue polling.

**Timeout:** If 15 minutes pass without the gate completing, report the current check states and the PR URL, then stop. Auto-merge remains enabled — no work is lost.

### Step 6a: Handle CI failures

If any check fails:

1. **Identify what failed.** Fetch the failed check details:
   ```bash
   gh pr checks <pr-number>
   gh run view <run-id> --log-failed
   ```

2. **Assess each failure:**
   - **Format/lint failure** (e.g., `dotnet format`, `swiftlint`, `npm run lint`): Fix it locally, commit, and push. These are mechanical fixes.
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

4. **Limit:** Make at most **3 rounds** of fixes. If the gate still hasn't passed after 3 rounds, report the outstanding failures and the PR URL, then stop. Auto-merge remains enabled.

### Step 7: Confirm merge and clean up

Once the gate passes, auto-merge will squash-merge the PR. Wait for it:

```bash
# Confirm the PR merged (poll briefly if needed)
gh pr view <pr-number> --json state -q '.state'
```

Once state is `"MERGED"`:

```bash
git checkout main
git pull origin main
```

**Full cleanup — worktrees, local branches, and remote branches:**

1. **Remove any worktrees** for the feature branch:
   ```bash
   # List worktrees and remove any pointing to the feature branch
   git worktree list
   # If the feature branch appears, remove its worktree
   git worktree remove <worktree-path>
   ```

2. **Delete the local branch:**
   ```bash
   git branch -D <branch-name>
   ```

3. **Delete the remote branch** (if GitHub's auto-delete-on-merge didn't catch it):
   ```bash
   git push origin --delete <branch-name>
   ```
   Ignore "remote ref does not exist" errors — that means auto-delete already handled it.

4. **Prune stale remote tracking refs:**
   ```bash
   git remote prune origin
   ```

5. **Verify** only `main` remains locally:
   ```bash
   git branch -a
   ```

### Step 8: Sync beads and verify

Push beads data to the Dolt remote — this is separate from git and must happen after every merge:

```bash
bd dolt push
```

Then run `git status` and `git log --oneline -3` to confirm:
- On `main`, up to date with origin
- The squash-merged commit is visible

Report success with the PR URL.

## Error Handling

- **Not on main:** Stop. Tell the user which branch they're on.
- **Nothing to ship:** Tell the user. Don't create empty PRs.
- **CI gate failed:** Report which checks failed, show relevant log output, and the PR URL. Fix mechanical issues (format, lint, tests) up to 3 rounds. For infrastructure/environment failures, stop and report.
- **Timeout (15 min, gate not complete):** Report current check states and PR URL. Auto-merge remains enabled — no work lost.
- **gh CLI not authenticated:** Tell the user to run `gh auth login`.
