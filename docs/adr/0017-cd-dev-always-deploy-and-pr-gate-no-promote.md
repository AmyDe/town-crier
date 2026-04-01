# 0017. CD Dev Always Deploys All Components; PR Gate No Longer Promotes

Date: 2026-04-01

## Status

Accepted (supersedes deployment aspects of [0015](0015-cicd-pipeline-and-deployment-strategy.md))

## Context

The CI/CD pipeline had two issues causing the dev environment to drift from main:

1. **PR Gate promoted API staging revisions to live traffic in dev.** This meant dev ran unmerged PR code after gate completion, and CD Dev redundantly rebuilt and redeployed the same code after merge.

2. **CD Dev used change detection (`HEAD~1..HEAD`) to conditionally deploy only changed components.** This meant web-only or API-only merges left other components potentially out of sync. Combined with a GitHub Actions trigger gap (auto-merge via `GITHUB_TOKEN` can suppress downstream workflow triggers), some merges failed to deploy to dev at all.

## Decision

- **PR Gate**: Remove the `api-promote` step. The staging revision is deployed and integration-tested for validation, then deactivated after tests complete. Dev traffic is never shifted during a PR.

- **CD Dev**: Remove change detection. Every push to main unconditionally deploys infrastructure (shared + dev stacks), API (build image + deploy), and web (build + deploy to SWA). This keeps dev in guaranteed sync with main.

- **Auto-merge**: Require a fine-grained PAT (`GH_AUTO_MERGE_TOKEN`) instead of `GITHUB_TOKEN` so the merge push triggers downstream workflows.

- **GitHub Environments**: Configure deployment branch policies — production restricted to `main` branch and `v*` tags; development has no branch restrictions (PR staging must deploy from any branch).

## Consequences

- Dev environment always reflects the latest state of main after every merge.
- PR Gate no longer affects live dev traffic, eliminating orphaned staging revisions.
- CD Dev runs take longer per merge (always builds API image + deploys web even if unchanged), but dev consistency is more valuable than saving a few minutes of CI time.
- A fine-grained PAT must be created and stored as `GH_AUTO_MERGE_TOKEN` for auto-merge to work and trigger CD Dev.
