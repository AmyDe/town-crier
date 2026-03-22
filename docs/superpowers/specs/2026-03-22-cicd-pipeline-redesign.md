# CI/CD Pipeline Redesign

Date: 2026-03-22

## Context

The current CI/CD setup has 7 workflow files with overlapping responsibilities. Tests run redundantly on both PR and main push. The web pipeline deploys directly to production on main merge. There is no unified dev deployment. The API image gets pushed to ACR but never deployed to the dev Container App.

The goals are:

1. **PR gate**: path-filtered, test/lint/format only (plus infra preview)
2. **Main merge**: path-filtered deploy to dev environment — no redundant testing
3. **Version tag**: deploy to production (already working)
4. **Build once**: API image is built and pushed to ACR on main merge, then promoted to prod by tag — no rebuild

## Design

### Workflow inventory

| File | Trigger | Purpose |
|------|---------|---------|
| `pr-gate.yml` | `pull_request` → main | Test, lint, format, infra preview |
| `cd-dev.yml` | `push` → main | Path-filtered deploy to dev |
| `cd-prod.yml` | `push` → `v*` tag | Full deploy to prod |

### Files to delete

| File | Reason |
|------|--------|
| `api-ci.yml` | Tests redundant with pr-gate; image build + deploy moves to cd-dev |
| `web-ci.yml` | Tests redundant with pr-gate; deploy moves to cd-dev; PR previews dropped |
| `ios-ci.yml` | Tests redundant with pr-gate; no server-side deploy for iOS |
| `infra-ci.yml` | Folded into cd-dev |
| `infra-shared-ci.yml` | Folded into cd-dev |

### pr-gate.yml — minor path-detection update

Runs on `pull_request` to main with path detection. Functionally unchanged, but the path patterns in the change-detection step must be updated to reference the new workflow files (`cd-dev.yml`, `pr-gate.yml`) instead of the deleted ones (`api-ci.yml`, `web-ci.yml`, `ios-ci.yml`, `infra-ci.yml`, `infra-shared-ci.yml`).

- **API** (when `api/**` changes): format check, build & test
- **iOS** (when `mobile/ios/**` changes): SwiftLint, build & test
- **Web** (when `web/**` changes): type-check, build
- **Infra** (when `infra/**` changes): Pulumi preview with PR comment
- **Gate job**: single required status check, passes if all triggered jobs pass or are skipped

Updated path patterns:

| Flag | Paths |
|------|-------|
| `api` | `api/*`, `.github/workflows/cd-dev.yml`, `.github/workflows/pr-gate.yml` |
| `ios` | `mobile/ios/*`, `.github/workflows/pr-gate.yml` |
| `web` | `web/*`, `.github/workflows/cd-dev.yml`, `.github/workflows/pr-gate.yml` |
| `infra` | `infra/*`, `.github/workflows/cd-dev.yml`, `.github/workflows/pr-gate.yml`, `.github/workflows/cd-prod.yml` |

PR preview environments for the web app are dropped. The dev environment (deployed on main merge) serves as the preview.

### cd-dev.yml — new workflow

Triggers on `push` to `main`. Uses git diff to detect which components changed. All deploy jobs use `environment: development`.

**Prerequisite**: The repo must use **squash merges** for PRs so that each push to main is a single commit. This ensures `HEAD~1` path detection works correctly.

**Permissions**: Requires `contents: read` and `id-token: write` (for Azure OIDC login) at the workflow level.

```
push to main
  │
  ├─ detect changes
  │
  ├─ infra/** changed?
  │   ├─ infra-shared: pulumi up (shared stack)
  │   └─ infra-dev: pulumi up (dev stack, waits for infra-shared)
  │
  ├─ api/** changed?
  │   ├─ api-image: docker build + push to ACR as <sha> (no tests)
  │   └─ api-deploy: az containerapp update on ca-town-crier-api-dev
  │       (waits for api-image; also waits for infra-dev if infra ran)
  │
  └─ web/** changed?
      └─ web-deploy: npm ci, npm run build, deploy to dev SWA
          (waits for infra-dev if infra ran)
```

#### Path detection

The change detection compares the previous commit against HEAD (since this is a push to main after a squash merge, each push is a single commit):

```yaml
CHANGED=$(git diff --name-only HEAD~1 HEAD)
```

The path patterns match the same categories as pr-gate:

| Flag | Paths |
|------|-------|
| `api` | `api/**`, `.github/workflows/cd-dev.yml` |
| `web` | `web/**`, `.github/workflows/cd-dev.yml` |
| `infra` | `infra/**`, `.github/workflows/cd-dev.yml` |

Self-referencing the workflow file ensures a change to `cd-dev.yml` itself triggers all components.

#### Infra jobs

Two sequential jobs when `infra/**` changes:

1. **infra-shared**: `pulumi up` on `shared` stack (ACR, CAE, managed identity)
2. **infra-dev**: `pulumi up` on `dev` stack (Cosmos DB, Container App, SWA) — `needs: infra-shared`

Both use `environment: development` and Azure OIDC login.

#### API jobs

Two sequential jobs when `api/**` changes:

1. **api-image**: Docker build and push to shared ACR, tagged with `github.sha` and `latest`. No tests — already passed in PR gate.
2. **api-deploy**: Azure OIDC login, then `az containerapp update` on `ca-town-crier-api-dev` in resource group `rg-town-crier-dev`, pointing to the new image tag. `needs: [changes, api-image, infra-dev]` with `if: !failure() && !cancelled() && needs.changes.outputs.api == 'true'`.

#### Web job

One job when `web/**` changes:

1. **web-deploy**: Azure OIDC login, `npm ci && npm run build`, then dynamically fetch the SWA deployment token via `az staticwebapp secrets list` and deploy. `needs: [changes, infra-dev]` with `if: !failure() && !cancelled() && needs.changes.outputs.web == 'true'`. Uses the same dynamic token approach as cd-prod.yml for consistency (no static `AZURE_STATIC_WEB_APPS_API_TOKEN` secret needed).

#### Cross-component dependencies

When both infra and API/web change in the same push, deploy jobs must wait for infra to finish (the Container App or SWA might not exist yet, or might be mid-update).

GitHub Actions does not support conditional `needs`. Instead, all deploy jobs statically list their full dependency chain in `needs`. When an upstream job is **skipped** (because its path-filter `if` evaluated to false), downstream jobs are also skipped by default. To prevent this cascade, deploy jobs must use an `if` condition that tolerates skipped dependencies:

```yaml
api-deploy:
  needs: [changes, api-image, infra-dev]
  if: >-
    !failure() && !cancelled()
    && needs.changes.outputs.api == 'true'
```

The `!failure() && !cancelled()` pattern allows the job to run when upstream jobs either succeeded or were skipped, but still blocks on actual failures. Each deploy job combines this with its own path-change condition.

The full static dependency graph:

| Job | `needs` | Runs when |
|-----|---------|-----------|
| infra-shared | changes | infra changed |
| infra-dev | changes, infra-shared | infra changed |
| api-image | changes | api changed |
| api-deploy | changes, api-image, infra-dev | api changed, image built, infra done or skipped |
| web-deploy | changes, infra-dev | web changed, infra done or skipped |

### cd-prod.yml — minor change: guard API deploy

Already triggers on `v*` tag push. Deploys:

1. **Infra**: `pulumi up` on `prod` stack
2. **API**: resolves tag to commit SHA, checks whether `town-crier-api:<sha>` exists in ACR before attempting deploy. If the image exists, runs `az containerapp update` on `ca-town-crier-api-prod`. If it doesn't exist (the tagged commit didn't change the API), skips gracefully. This is necessary because only API-changing commits produce images in the new model.
3. **Web**: `npm ci && npm run build`, deploy to prod SWA

Uses `environment: production`.

The image existence check can be done with:

```bash
az acr manifest show --registry "$ACR_NAME" --name "town-crier-api:$SHA" 2>/dev/null
```

If this returns non-zero, the API deploy step is skipped.

**Note on shared infra**: The `shared` Pulumi stack (ACR, CAE, managed identity) is deployed only by `cd-dev.yml` on main merge, not by `cd-prod.yml`. This is intentional — shared resources are environment-agnostic and will always be updated before any `v*` tag is pushed.

### API image lifecycle

```
PR branch    →  build + test (no image)
main merge   →  docker build + push to ACR as <sha> + latest
v* tag       →  resolve tag to SHA, deploy existing image to prod
```

The tag points to a commit on main, so the SHA matches an image already in ACR. No rebuild for prod.

### GitHub environment setup

Two GitHub environments needed:

- **development**: no protection rules (free tier), used by cd-dev
- **production**: no protection rules (free tier limitation), used by cd-prod; tag-based triggering serves as the manual gate

### Concurrency

| Workflow | Concurrency group | Cancel in-progress |
|----------|------------------|--------------------|
| pr-gate | `pr-gate-<ref>` | yes |
| cd-dev | `cd-dev` | no (let deploys finish) |
| cd-prod | `cd-prod` | no (let deploys finish) |

## Prerequisites

Before the new workflows will function, these manual steps are required:

1. **Configure squash merges**: Ensure the repo is set to squash-merge PRs (Settings → General → Pull Requests). The `HEAD~1` path detection in cd-dev depends on each push to main being a single commit.
2. **Create `development` GitHub environment**: Settings → Environments → New environment → "development". No protection rules needed.
3. **Add OIDC federated credential for `development` environment**: The existing Azure AD app registration has a federated credential scoped to `environment:production`. A second credential must be added for `environment:development` (same subject format, different environment value). Without this, all cd-dev Azure logins will fail.
4. **Retire the `AZURE_STATIC_WEB_APPS_API_TOKEN` secret**: cd-dev uses dynamic token fetch via `az staticwebapp secrets list` (matching cd-prod). The static secret is no longer needed after the old `web-ci.yml` is deleted.

## Consequences

### What gets better

- No redundant test runs on main — faster feedback, lower CI costs
- Single source of truth for dev deployment (cd-dev.yml)
- API image built once, promoted to prod without rebuild
- Fewer workflow files (7 → 3)
- Consistent SWA deployment approach (dynamic token) across dev and prod

### What gets worse

- PR preview environments for web are gone — must merge to main to see the site live in dev
- If someone pushes directly to main (bypassing PR), no tests run on that code. Branch protection requiring PRs is assumed, even though the free tier can't enforce it with required status checks on environments.
- Consecutive merges to main queue in cd-dev (cancel-in-progress is false) — could be slow if infra changes are in the queue (15-minute timeout)

### Risks

- The `development` GitHub environment and OIDC credential must be created before switching workflows (see Prerequisites)
- If squash merge is not enforced, `HEAD~1` path detection may produce incorrect results for merge commits
