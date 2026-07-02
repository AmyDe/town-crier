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

**Preferred: run `scripts/wf/next-version.sh [patch|minor|major]`** — with no argument it auto-detects the bump from conventional commits since the latest tag (`type!:`/`BREAKING CHANGE` → major, any `feat` → minor, else patch) and prints the next `vX.Y.Z`. The manual rules below are the fallback and the explanation:

Apply semver rules to the current version:
- **patch**: `vX.Y.Z` → `vX.Y.(Z+1)`
- **minor**: `vX.Y.Z` → `vX.(Y+1).0`
- **major**: `vX.Y.Z` → `v(X+1).0.0`

### Step 3: Collect and categorize commits

**Preferred: run `scripts/wf/release-notes.sh [previous-tag]`** — it emits the categorized markdown skeleton (Features / Bug Fixes / Performance / … / Other) in the exact format Step 4 describes, grouping every `type(scope): desc (#PR)` commit. Add the optional dash-explanations by hand where a subject alone isn't clear, then use it as the release body. The manual rules below are the fallback:

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

**Bead DB cleanup (a release is a natural checkpoint).** Check size with `bd stats`. If **total issues exceed ~1000** (or `bd` commands feel sluggish), compact the DB before finishing:

- **`bd flatten --force`** — squash Dolt commit history (server mode writes one commit per write via `dolt.auto-commit: on`). Main speed lever + reclaims disk + runs Dolt GC. Prefer over `bd compact`, whose commit-squash can fail with a Dolt constraint violation on a churned DB. After flatten, the next `bd dolt push` may need `--force` (rewritten history).
- **`bd admin compact`** — semantic decay of old closed issues, run ~quarterly. Agent-driven, no API key: `bd compact --analyze --json` → write summaries → `bd compact --apply --id <id> --summary -`.

The Dolt DB is the source of truth — never hand-edit `.beads/issues.jsonl` to "clean up"; `bd` regenerates it and the edits get clobbered.

### Step 7: Close resolved GitHub issues

**Run `scripts/wf/close-released-issues.sh <version> <release-url>`.** It sweeps open issues labelled `bead-created`, extracts linked `tc-*` bead IDs from their triage comments, and closes an issue only when *every* linked bead is `closed` — leaving alone any issue with an open/in-progress bead, an unresolvable bead, or no discoverable linkage (surfaced for manual review). It prints a one-line tally (`Closed N GH issue(s) …` or `No GH issues ready to close.`).

This step is best-effort — failures here must not roll back the release. The `bead-created` label is a legacy triage marker on older issues, so the sweep is a no-op when nothing matches.

## Edge cases

- **No commits since last tag**: Tell the user there's nothing to release and stop.
- **Dirty working tree**: This skill creates releases from what's already on main — local uncommitted changes aren't relevant. Note this to the user if `git status` shows changes, but proceed with the release.
- **Tag already exists**: If the computed version tag already exists on the remote, tell the user and ask how to proceed (skip, force, or pick a different version).
- **Missing `gh` CLI**: The `gh` CLI is required. If not available, tell the user to install it.
