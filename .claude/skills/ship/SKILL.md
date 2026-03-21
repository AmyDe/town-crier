---
name: ship
description: Automate the push-to-main flow when you have local commits and/or unstaged changes on main. Creates a feature branch, opens a PR via `gh`, and waits for PR gate checks before merging. MUST use this skill whenever the user says "ship", "ship it", "push to main", "push my changes", "get this on main", "merge to main", "create a PR and merge", or any variation of wanting to get local work from main onto the remote. Also trigger when the user has been working on main and wants to push but can't due to branch protection. Do NOT use for: creating PRs without merging, pushing feature branches, or work that isn't on main.
---

# Ship to Main

Route local work on `main` through a PR, because direct pushes to main are blocked by branch protection and a local pre-push hook. This skill handles the flow: branch, push, create PR, then ask the user whether to wait for checks or leave the PR open.

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

### Step 6: Ask user about merge strategy

**NEVER merge the PR immediately.** The PR gate workflow must pass first. Present the PR URL and ask:

> PR created: <url>
>
> The PR gate checks are running. Would you like me to:
> 1. **Wait** for the checks to pass and then merge
> 2. **Leave it open** for you to merge later

**If the user chooses to wait:**

Use `gh pr checks --watch` to monitor, then merge when green:

```bash
gh pr checks <pr-number> --watch --fail-fast
```

If checks pass, proceed to merge and cleanup (Step 7). If checks fail, report which checks failed, provide the PR URL, and stop — do not retry or force-merge.

**If the user chooses to leave it open:**

Report the PR URL and stop. Do not proceed to Step 7. Do not merge. Do not clean up branches. The skill ends here.

### Step 7: Merge and clean up

Only reach this step if the user chose to wait AND checks passed.

```bash
gh pr merge --squash --delete-branch
git checkout main
git pull origin main
```

### Step 8: Verify

Run `git status` and `git log --oneline -3` to confirm:
- On `main`, up to date with origin
- The squash-merged commit is visible

Report success with the PR URL.

## Error Handling

- **Not on main:** Stop. Tell the user which branch they're on.
- **Nothing to ship:** Tell the user. Don't create empty PRs.
- **Checks failed:** Report which checks failed and the PR URL. Do not retry or force-merge.
- **gh CLI not authenticated:** Tell the user to run `gh auth login`.
