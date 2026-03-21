# Shared ACR and Image Promotion Design

Date: 2026-03-21

## Problem

The current architecture creates a per-environment Azure Container Registry (ACR) in each Pulumi stack. This means:

1. The CD pipeline rebuilds the Docker image for production rather than promoting the image that was tested in dev — no guarantee of bit-for-bit parity.
2. Tearing down the dev stack would destroy the dev ACR, and tearing down the prod stack would destroy the prod ACR. There is no shared, lifecycle-independent registry.
3. Admin credentials are used for ACR authentication, requiring password management and rotation.

## Decision

Introduce a `shared` Pulumi stack that owns a single ACR and a user-assigned managed identity. Environment stacks (`dev`, `prod`) consume these via stack references. The CD pipeline promotes images by SHA reference instead of rebuilding.

## Design

### Shared Pulumi Stack

A new `shared` stack within the existing `/infra` Pulumi project. The program branches on the `environment` config value to determine which resources to create. The branching must happen early — before any environment-specific config reads (e.g., `cosmosConsistencyLevel`) — so that the shared stack only reads the config it needs.

**Resources:**

| Resource | Name | Purpose |
|----------|------|---------|
| Resource Group | `rg-town-crier-shared` | Isolates shared resources for cost tracking |
| Container Registry | `acrtowncriershared` | Single registry for all environments (Basic SKU, admin auth disabled) |
| User-Assigned Managed Identity | `id-town-crier-acr-pull` | Identity with `AcrPull` role on the ACR |
| Role Assignment | `AcrPull` on ACR | Grants the managed identity pull access to the registry |
| Role Assignment | `AcrPush` on ACR | Grants the GitHub Actions service principal push access to the registry |

The `AcrPush` role assignment is needed because CI (`api-ci.yml`) pushes images to the shared ACR. The service principal used by GitHub Actions OIDC needs push access. The principal ID can be derived from the `AZURE_CLIENT_ID` passed as a Pulumi config value (e.g., `town-crier:ciServicePrincipalId`).

**Stack config** (`Pulumi.shared.yaml`):

```yaml
config:
  azure-native:location: uksouth
  town-crier:environment: shared
  town-crier:ciServicePrincipalId: <object-id-of-github-actions-sp>
```

**Exported outputs** (consumed by dev/prod via stack references):

- `containerRegistryLoginServer` — e.g., `acrtowncriershared.azurecr.io`
- `acrPullIdentityId` — the full resource ID of the managed identity
- `acrPullIdentityClientId` — the client ID, needed for Container App registry config

### Changes to Dev/Prod Stacks

Each environment stack changes in three ways:

1. **ACR removed** — the `Registry`, admin credentials lookup (`ListRegistryCredentials`), `containerRegistryLoginServer` output, and `acr-password` secret are all deleted.

2. **Stack reference added** — reads shared stack outputs. The Pulumi Cloud organization is `AmyDe` (confirmed via `pulumi whoami`):
   ```csharp
   var shared = new StackReference("AmyDe/town-crier/shared");
   var acrLoginServer = shared.GetOutput("containerRegistryLoginServer");
   var acrPullIdentityId = shared.GetOutput("acrPullIdentityId");
   var acrPullIdentityClientId = shared.GetOutput("acrPullIdentityClientId");
   ```

3. **Container App switches to managed identity auth** — the user-assigned identity is attached to the Container App and referenced in the registry config. Specifically:
   - Add `Identity` to the Container App with `Type = ManagedServiceIdentityType.UserAssigned` and the identity resource ID in the `UserAssignedIdentities` map
   - Replace `RegistryCredentialsArgs` to use `Server` + `Identity` (the managed identity resource ID) instead of `Server` + `Username` + `PasswordSecretRef`
   - Remove the `acr-password` entry from the `Secrets` array entirely

### CI/CD Workflow Changes

#### `api-ci.yml` (dev — push to main)

No workflow file changes. The existing `secrets.ACR_LOGIN_SERVER` secret is updated to point to the shared ACR. The `az acr login --name` command accepts both registry names and FQDNs, so the existing call works without modification. Images continue to be tagged with `github.sha` and `latest`.

#### `cd-prod.yml` (prod — version tags)

The API job is simplified from build-and-push to promote-by-reference:

1. Resolve the commit SHA from the version tag: `git rev-parse $GITHUB_REF_NAME`
2. Update the prod Container App image to `${{ secrets.ACR_LOGIN_SERVER }}/town-crier-api:<sha>`

No Docker build, no image push, no ACR login. The `az containerapp update` command only updates the Container App configuration — the actual image pull happens at runtime using the Container App's managed identity. The ACR login step and Docker build/push steps are removed from this workflow.

The image was already pushed to the shared ACR by `api-ci.yml` when the code merged to main.

#### `infra-shared-ci.yml` (new)

A new workflow for the shared stack:

- **Trigger:** Push to `main` with path filter on `infra/**`
- **Action:** `pulumi up --stack shared`
- **Frequency:** Rarely runs — only when ACR or identity config changes

#### Workflow ordering on `infra/**` changes

Both `infra-shared-ci.yml` and `infra-ci.yml` (dev) trigger on pushes to `main` with the `infra/**` path filter. Since the dev stack depends on the shared stack via a stack reference, the shared stack must deploy first.

`infra-ci.yml` is updated to use a `workflow_run` trigger that waits for `infra-shared-ci.yml` to complete, rather than triggering directly on push. This ensures the shared stack is always up-to-date before the dev stack runs `pulumi up`.

### Secret and Environment Changes

| Item | Change |
|------|--------|
| `ACR_LOGIN_SERVER` (repo secret) | Update value to `acrtowncriershared.azurecr.io` |
| `PULUMI_STACK` (repo variable) | Stays as `dev` |
| OIDC federated credentials | No changes — `ref:refs/heads/main` covers shared/dev, `environment:production` covers prod |
| New secrets | None — managed identity eliminates ACR passwords |

### Image Tagging Strategy

```
push to main (api/ changes):
  api-ci.yml → docker build + push
    → acrtowncriershared.azurecr.io/town-crier-api:<commit-sha>
    → acrtowncriershared.azurecr.io/town-crier-api:latest

git tag v1.0.0 + push:
  cd-prod.yml → resolve tag to SHA → az containerapp update
    → uses acrtowncriershared.azurecr.io/town-crier-api:<commit-sha>
```

The same SHA-tagged image that ran in dev is promoted to prod. No `:latest` or version tags needed in prod — the SHA is the canonical identifier.

### Rollback

To roll back a prod deployment, run `az containerapp update` with the previous commit SHA. The old image remains in the shared ACR (tagged by SHA). No special mechanism is needed — it's the same promote-by-reference operation.

ACR Basic SKU has 10 GiB storage. SHA-tagged images will accumulate over time. An ACR purge task should be configured to retain the last 30 tagged images and delete untagged manifests.

## Migration Path

The migration is performed in this order:

1. **Deploy shared stack** — creates the shared ACR, managed identity, and role assignments. No impact on existing resources.
2. **Update `ACR_LOGIN_SERVER` secret** — point to the shared ACR. From this point, `api-ci.yml` pushes to the shared registry.
3. **Push an image** — trigger `api-ci.yml` so that at least one image exists in the shared ACR before the Container Apps are updated to reference it.
4. **Update dev stack** — remove per-env ACR, add stack reference, switch Container App to managed identity auth. Pulumi will destroy the old dev ACR and update the Container App configuration.
5. **Verify dev** — confirm the Container App starts and can pull from the shared ACR.
6. **Update prod stack** — same changes as dev. Pulumi will destroy the old prod ACR on next deploy.
7. **Update `cd-prod.yml`** — switch from Docker build/push to image promotion.

## Consequences

**What becomes easier:**

- Prod deployments are faster — no Docker build, just a Container App image reference update
- Bit-for-bit parity between dev and prod images is guaranteed
- No ACR passwords to manage or rotate
- Shared resources have an independent lifecycle — tearing down dev or prod doesn't affect the registry
- Cost tracking remains clean — shared, dev, and prod each have their own resource group

**What becomes more difficult:**

- Initial setup requires deploying the shared stack before dev/prod
- The Pulumi program needs branching logic to handle the `shared` environment differently from `dev`/`prod`
- One extra stack to be aware of (though it rarely changes)
- Workflow ordering between shared and dev infra deploys adds a dependency chain
