---
name: release
description: Create a new GitHub release with categorized release notes from conventional commits. MUST use this skill whenever the user says "release", "new release", "cut a release", "release a patch", "release a minor", "release a major", "bump the version", "new version", "tag a release", "ship a release", or any variation of wanting to create a versioned release. Also trigger when the user mentions "patch version", "minor version", or "major version" in a release context. Do NOT use for: creating PRs, deploying code, or shipping local changes to main (use the ship skill for that).
---

# Release

Create a versioned GitHub release with categorized release notes derived from conventional commits. Uses `gh release create` to create the tag directly on GitHub — never `git tag` + `git push`, because the branch protection hook blocks pushes from main.

## Determining the bump level

The user's request tells you which component to bump. Map their intent:

| User says | Bump |
|-----------|------|
| "patch", "bugfix release", "point release", just "release" with no qualifier | patch |
| "minor", "feature release" | minor |
| "major", "breaking release" | major |

If the request is ambiguous (e.g., just "release" or "new version"), look at the commits since the last tag:
- If there are any `feat` commits → default to **minor**
- If there are only `fix`, `docs`, `chore`, `perf`, `ci`, `refactor`, `test` commits → default to **patch**
- If any commit message contains `BREAKING CHANGE` or uses `!` after the type (e.g., `feat!:`) → default to **major**

Proceed automatically with the computed version — no confirmation needed.

## Workflow

### Step 1: Find the current version

```bash
git fetch --tags origin
git tag --sort=-v:refname | head -5
```

Parse the latest `vX.Y.Z` tag. If no tags exist, start from `v0.0.0`.

### Step 2: Compute the next version

Apply semver rules to the current version:
- **patch**: `vX.Y.Z` → `vX.Y.(Z+1)`
- **minor**: `vX.Y.Z` → `vX.(Y+1).0`
- **major**: `vX.Y.Z` → `v(X+1).0.0`

### Step 3: Collect and categorize commits

```bash
git log <previous-tag>..HEAD --oneline
```

Parse each commit using conventional commit format: `type(scope): description (#PR)`. Group into these categories, in this order — omit any category with no commits:

| Conventional type | Release notes heading |
|---|---|
| `feat` | Features |
| `fix` | Bug Fixes |
| `perf` | Performance |
| `refactor` | Refactoring |
| `test` | Testing |
| `ci`, `chore` (CI-related) | CI/CD |
| `docs` | Documentation |
| `chore` (non-CI) | Chores |

Commits that don't follow conventional commit format go under **Other**.

### Step 4: Format the release notes

Use this exact template — it matches the project's established release format:

```
## What's Changed

### Features
- **feat(scope): short description** (#123) — Optional longer explanation

### Bug Fixes
- **fix(scope): short description** (#456) — Optional longer explanation
```

Rules:
- Bold the commit message prefix and description
- Include the PR number as a link fragment `(#N)`
- Add a dash-separated explanation only when the commit title alone isn't clear enough
- Omit the `(scope)` part if the original commit had no scope
- Prefix `ci` and `chore` entries with the type in lowercase (e.g., `chore(ci): ...`) rather than bolding them, to match the established style for those categories

### Step 5: Create the release

```bash
gh release create <version> \
  --target main \
  --title "<version>" \
  --notes "<release-notes>"
```

Use a heredoc for the notes to preserve formatting:

```bash
gh release create v0.7.0 --target main --title "v0.7.0" --notes "$(cat <<'EOF'
## What's Changed

### Features
- **feat(web): add dark mode support** (#160) — Respects system preference by default
EOF
)"
```

After creation, report the release URL to the user.

### Step 6: Sync beads

If there are any open beads that were completed by the released commits, offer to close them. Then run `bd dolt push` to sync beads state.

### Step 7: Close resolved GitHub issues

Sweep open GitHub issues whose linked beads are all closed, and close them with a pointer to the release. The `bead-created` label (added by the `triage-inbox` skill) marks issues that have been converted into beads; the linkage lives in a triage comment of the form `Triaged → bead **tc-xxxx**` (and optionally `, child tasks: tc-aaaa, tc-bbbb`).

```bash
gh issue list --state=open --label=bead-created --limit=200 --json=number,title,url
```

If empty, skip the step.

For each issue:

1. **Extract linked bead IDs** from the comment trail:
   ```bash
   gh issue view <number> --json comments --jq '[.comments[].body] | join(" ")' \
     | grep -oE 'tc-[a-z0-9]+' | sort -u
   ```
2. **Skip if no bead IDs are found.** Don't close issues whose linkage can't be confirmed — surface them for manual review instead.
3. **Check each bead's status:**
   ```bash
   bd show <bead-id> --json | jq -r '.[0].status'
   ```
   Treat a missing bead (no JSON returned) as still-open and skip the issue — a deleted bead shouldn't auto-close an issue.
4. **If every linked bead is `closed`**, close the GitHub issue:
   ```bash
   gh issue close <number> --comment "All linked beads are closed; shipped in [<version>](<release-url>)."
   ```
   If at least one linked bead is still `open` or `in_progress`, leave the issue alone.

Report a one-line tally:
```
Closed 3 GH issue(s) whose beads all shipped: #358, #360, #362
```
or
```
No GH issues ready to close.
```

This step is best-effort — failures here must not roll back the release. Log and continue.

## Edge cases

- **No commits since last tag**: Tell the user there's nothing to release and stop.
- **Dirty working tree**: This skill creates releases from what's already on main — local uncommitted changes aren't relevant. Note this to the user if `git status` shows changes, but proceed with the release.
- **Tag already exists**: If the computed version tag already exists on the remote, tell the user and ask how to proceed (skip, force, or pick a different version).
- **Missing `gh` CLI**: The `gh` CLI is required. If not available, tell the user to install it.
