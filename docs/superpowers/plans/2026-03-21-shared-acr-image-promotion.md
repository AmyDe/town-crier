# Shared ACR and Image Promotion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace per-environment ACRs with a shared ACR using managed identity auth, and switch prod deployments from Docker rebuild to image promotion by SHA.

**Architecture:** A new `shared` Pulumi stack owns the ACR and a user-assigned managed identity. The `dev`/`prod` stacks consume these via stack references and attach the identity to their Container Apps. The CD pipeline promotes images by referencing existing SHA-tagged images instead of rebuilding.

**Tech Stack:** Pulumi (.NET/C#), Azure Container Registry, Azure Managed Identity, GitHub Actions

**Spec:** `docs/superpowers/specs/2026-03-21-shared-acr-image-promotion-design.md`

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `infra/Pulumi.shared.yaml` | Stack config for shared environment |
| Create | `infra/SharedStack.cs` | Shared stack resources (ACR, identity, roles) |
| Create | `infra/EnvironmentStack.cs` | Environment stack resources (everything currently in Program.cs) |
| Modify | `infra/Program.cs` | Branch on environment, delegate to shared or env stack |
| Create | `.github/workflows/infra-shared-ci.yml` | CI/CD for shared Pulumi stack |
| Modify | `.github/workflows/infra-ci.yml` | Switch to `workflow_run` trigger after shared |
| Modify | `.github/workflows/cd-prod.yml` | Remove Docker build, promote image by SHA |
| Modify | `.github/workflows/pr-gate.yml` | Add `infra-shared-ci.yml` to path detection |

---

### Task 1: Create shared stack config and skeleton

**Files:**
- Create: `infra/Pulumi.shared.yaml`
- Create: `infra/SharedStack.cs`
- Modify: `infra/Program.cs:12-16` — add early branch on environment

- [ ] **Step 1: Create `Pulumi.shared.yaml`**

```yaml
config:
  azure-native:location: uksouth
  town-crier:environment: shared
  town-crier:ciServicePrincipalId: 8efcb7cf-f17e-4a93-aab5-df7bc3c2c2cc
```

- [ ] **Step 2: Create `SharedStack.cs` with empty method**

```csharp
using Pulumi;

public static class SharedStack
{
    public static Dictionary<string, object?> Run(Config config, InputMap<string> tags)
    {
        return new Dictionary<string, object?>();
    }
}
```

- [ ] **Step 3: Refactor `Program.cs` to branch on environment**

The key change: read `environment` first, and only read `cosmosConsistencyLevel` inside the env branch. Move all current resource code into `EnvironmentStack.cs`.

```csharp
using Pulumi;

return await Pulumi.Deployment.RunAsync(() =>
{
    var config = new Config("town-crier");
    var env = config.Require("environment");

    var tags = new InputMap<string>
    {
        { "project", "town-crier" },
        { "managedBy", "pulumi" },
        { "environment", env },
    };

    if (env == "shared")
    {
        return SharedStack.Run(config, tags);
    }

    return EnvironmentStack.Run(config, env, tags);
});
```

- [ ] **Step 4: Create `EnvironmentStack.cs`**

Move all existing resource code from `Program.cs` (lines 16-372) into a static method. The method receives `config`, `env`, and `tags`. The `cosmosConsistencyLevel` read moves here.

```csharp
using System.Collections.Immutable;
using Pulumi;
using Pulumi.AzureNative.Resources;
using Pulumi.AzureNative.OperationalInsights;
using Pulumi.AzureNative.App;
using Pulumi.AzureNative.App.Inputs;
using Pulumi.AzureNative.CosmosDB;
using Pulumi.AzureNative.CosmosDB.Inputs;
using Pulumi.AzureNative.ContainerRegistry;
using Pulumi.AzureNative.Web;

public static class EnvironmentStack
{
    public static Dictionary<string, object?> Run(Config config, string env, InputMap<string> tags)
    {
        var cosmosConsistencyLevel = config.Require("cosmosConsistencyLevel");

        // [All existing resource code from Program.cs lines 25-371, unchanged]
        // Resource Group, Log Analytics, Container Apps Environment,
        // ACR, Cosmos DB, Container App, Static Web App, return outputs
    }
}
```

Copy the full body of the current `Program.cs` (from the resource group through the return statement) verbatim into this method. The only change is that `config`, `env`, and `tags` are now parameters.

- [ ] **Step 5: Verify build**

Run: `cd /Users/christy/Dev/town-crier/infra && dotnet build`
Expected: Build succeeded

- [ ] **Step 6: Verify dev stack preview is unchanged**

Run: `cd /Users/christy/Dev/town-crier/infra && pulumi preview --stack dev --diff`
Expected: No changes (the refactoring should be behavior-preserving)

- [ ] **Step 7: Commit**

```bash
git add infra/Pulumi.shared.yaml infra/SharedStack.cs infra/EnvironmentStack.cs infra/Program.cs
git commit -m "refactor: extract shared and environment stack entry points"
```

---

### Task 2: Implement shared stack resources

**Files:**
- Modify: `infra/SharedStack.cs`

- [ ] **Step 1: Add shared stack resources**

Replace the empty `SharedStack.Run` with the full implementation:

```csharp
using Pulumi;
using Pulumi.AzureNative.Resources;
using Pulumi.AzureNative.ContainerRegistry;
using Pulumi.AzureNative.ManagedIdentity;
using Pulumi.AzureNative.Authorization;

public static class SharedStack
{
    public static Dictionary<string, object?> Run(Config config, InputMap<string> tags)
    {
        var ciServicePrincipalId = config.Require("ciServicePrincipalId");

        // Resource Group
        var resourceGroup = new ResourceGroup("rg-town-crier-shared", new ResourceGroupArgs
        {
            ResourceGroupName = "rg-town-crier-shared",
            Tags = tags,
        });

        // Container Registry (admin auth disabled — managed identity only)
        var containerRegistry = new Registry("acrtowncriershared", new RegistryArgs
        {
            RegistryName = "acrtowncriershared",
            ResourceGroupName = resourceGroup.Name,
            Sku = new Pulumi.AzureNative.ContainerRegistry.Inputs.SkuArgs
            {
                Name = SkuName.Basic,
            },
            AdminUserEnabled = false,
            Tags = tags,
        });

        // User-Assigned Managed Identity for AcrPull
        var acrPullIdentity = new UserAssignedIdentity("id-town-crier-acr-pull", new UserAssignedIdentityArgs
        {
            ResourceName = "id-town-crier-acr-pull",
            ResourceGroupName = resourceGroup.Name,
            Tags = tags,
        });

        // AcrPull role assignment — managed identity can pull images
        var acrPullRole = new RoleAssignment("acr-pull-role", new RoleAssignmentArgs
        {
            Scope = containerRegistry.Id,
            RoleDefinitionId = containerRegistry.Id.Apply(acrId =>
            {
                var subscriptionId = acrId.Split('/')[2];
                return $"/subscriptions/{subscriptionId}/providers/Microsoft.Authorization/roleDefinitions/7f951dda-4ed3-4680-a7ca-43fe172d538d";
            }),
            PrincipalId = acrPullIdentity.PrincipalId,
            PrincipalType = "ServicePrincipal",
        });

        // AcrPush role assignment — GitHub Actions SP can push images
        var acrPushRole = new RoleAssignment("acr-push-role", new RoleAssignmentArgs
        {
            Scope = containerRegistry.Id,
            RoleDefinitionId = containerRegistry.Id.Apply(acrId =>
            {
                var subscriptionId = acrId.Split('/')[2];
                return $"/subscriptions/{subscriptionId}/providers/Microsoft.Authorization/roleDefinitions/8311e382-0749-4cb8-b61a-304f252e45ec";
            }),
            PrincipalId = ciServicePrincipalId,
            PrincipalType = "ServicePrincipal",
        });

        return new Dictionary<string, object?>
        {
            ["resourceGroupName"] = resourceGroup.Name,
            ["containerRegistryLoginServer"] = containerRegistry.LoginServer,
            ["acrPullIdentityId"] = acrPullIdentity.Id,
            ["acrPullIdentityClientId"] = acrPullIdentity.ClientId,
        };
    }
}
```

**Notes on role definition IDs:**
- `7f951dda-4ed3-4680-a7ca-43fe172d538d` = AcrPull built-in role
- `8311e382-0749-4cb8-b61a-304f252e45ec` = AcrPush built-in role

- [ ] **Step 2: Verify build**

Run: `cd /Users/christy/Dev/town-crier/infra && dotnet build`
Expected: Build succeeded

- [ ] **Step 3: Preview the shared stack**

Run: `cd /Users/christy/Dev/town-crier/infra && pulumi preview --stack shared --diff`
Expected: Plan to create 5 resources (resource group, ACR, identity, 2 role assignments)

- [ ] **Step 4: Deploy the shared stack**

Run: `cd /Users/christy/Dev/town-crier/infra && pulumi up --stack shared --yes`
Expected: 5 resources created successfully

- [ ] **Step 5: Verify outputs**

Run: `cd /Users/christy/Dev/town-crier/infra && pulumi stack output --stack shared --json`
Expected: JSON with `containerRegistryLoginServer`, `acrPullIdentityId`, `acrPullIdentityClientId`, `resourceGroupName`

- [ ] **Step 6: Commit**

```bash
git add infra/SharedStack.cs
git commit -m "feat: add shared stack with ACR, managed identity, and role assignments"
```

---

### Task 3: Update environment stack to use shared ACR with managed identity

**Files:**
- Modify: `infra/EnvironmentStack.cs` — remove ACR, add stack reference, switch Container App to managed identity

- [ ] **Step 1: Add using statements**

Add to the top of `EnvironmentStack.cs`:

```csharp
using Pulumi.AzureNative.App.Inputs;  // already present
// Add:
using ManagedServiceIdentityType = Pulumi.AzureNative.App.ManagedServiceIdentityType;
```

- [ ] **Step 2: Add stack reference after tags**

At the top of the `Run` method, after the `cosmosConsistencyLevel` read, add:

```csharp
// Shared stack outputs
var shared = new StackReference("AmyDe/town-crier/shared");
var acrLoginServer = shared.GetOutput("containerRegistryLoginServer").Apply(o => o?.ToString() ?? "");
var acrPullIdentityId = shared.GetOutput("acrPullIdentityId").Apply(o => o?.ToString() ?? "");
var acrPullIdentityClientId = shared.GetOutput("acrPullIdentityClientId").Apply(o => o?.ToString() ?? "");
```

- [ ] **Step 3: Remove ACR resources**

Delete the following blocks from `EnvironmentStack.cs`:
- The `containerRegistry` variable (Registry resource)
- The `acrCredentials` variable (ListRegistryCredentials call)
- Remove the `using Pulumi.AzureNative.ContainerRegistry;` import

- [ ] **Step 4: Update Container App to use managed identity**

Replace the Container App's `Configuration` block. The key changes:
- `Registries` uses `Identity` instead of `Username`/`PasswordSecretRef`
- `Secrets` array is removed entirely
- `Identity` property is added to attach the user-assigned managed identity

Replace the ContainerApp resource definition's `Configuration` with:

```csharp
Configuration = new ConfigurationArgs
{
    Ingress = new IngressArgs
    {
        External = true,
        TargetPort = 8080,
        Transport = IngressTransportMethod.Http,
    },
    Registries = new[]
    {
        new RegistryCredentialsArgs
        {
            Server = acrLoginServer,
            Identity = acrPullIdentityId,
        },
    },
},
```

Add the `Identity` property to the ContainerApp (at the same level as `Configuration`, `Template`, `Tags`):

```csharp
Identity = new ManagedServiceIdentityArgs
{
    Type = ManagedServiceIdentityType.UserAssigned,
    UserAssignedIdentities = new InputList<string>
    {
        acrPullIdentityId,
    },
},
```

**Note:** The `ManagedServiceIdentityArgs` type is in `Pulumi.AzureNative.App.Inputs`. The `ManagedServiceIdentityType` enum is in `Pulumi.AzureNative.App`.

- [ ] **Step 5: Update outputs**

Remove `containerRegistryLoginServer` from the return dictionary (it's now owned by the shared stack).

- [ ] **Step 6: Verify build**

Run: `cd /Users/christy/Dev/town-crier/infra && dotnet build`
Expected: Build succeeded

- [ ] **Step 7: Preview dev stack**

Run: `cd /Users/christy/Dev/town-crier/infra && pulumi preview --stack dev --diff`
Expected: Plan to delete old ACR + admin credentials, update Container App (identity + registry config change). Review the diff carefully — it should show the ACR being destroyed and the Container App being updated.

- [ ] **Step 8: Deploy dev stack**

Run: `cd /Users/christy/Dev/town-crier/infra && pulumi up --stack dev --yes`
Expected: Old ACR destroyed, Container App updated with managed identity auth

- [ ] **Step 9: Commit**

```bash
git add infra/EnvironmentStack.cs
git commit -m "feat: switch environment stacks to shared ACR with managed identity"
```

---

### Task 4: Update GitHub secret and push an image

**Files:** None (CLI operations only)

- [ ] **Step 1: Get the shared ACR login server**

Run: `cd /Users/christy/Dev/town-crier/infra && pulumi stack output containerRegistryLoginServer --stack shared`
Expected: `acrtowncriershared.azurecr.io`

- [ ] **Step 2: Update the GitHub secret**

Run: `gh secret set ACR_LOGIN_SERVER --repo AmyDe/town-crier --body "acrtowncriershared.azurecr.io"`
Expected: Secret updated

- [ ] **Step 3: Push commits so `api-ci.yml` runs and pushes an image to the shared ACR**

This happens naturally when we push the code changes. After the push, check that `api-ci.yml` runs and the image push succeeds. If `api-ci.yml` does not trigger (no changes in `api/`), manually trigger it or push a no-op change.

Run: `gh workflow view "API CI" --repo AmyDe/town-crier`
Expected: Verify latest run succeeded and pushed to the shared ACR

---

### Task 5: Create `infra-shared-ci.yml` workflow

**Files:**
- Create: `.github/workflows/infra-shared-ci.yml`

- [ ] **Step 1: Write the workflow**

```yaml
# Deploys the shared Pulumi stack (ACR, managed identity).
# Must complete before infra-ci.yml (dev stack) runs.
#
# Requires secrets:
#   AZURE_CLIENT_ID        — OIDC federated credential for Azure login
#   AZURE_TENANT_ID        — Azure AD tenant
#   AZURE_SUBSCRIPTION_ID  — Azure subscription
#   PULUMI_ACCESS_TOKEN    — Pulumi Cloud access token

name: Infrastructure Shared CI

on:
  push:
    branches: [main]
    paths:
      - "infra/**"
      - ".github/workflows/infra-shared-ci.yml"

concurrency:
  group: infra-shared-ci
  cancel-in-progress: false

permissions:
  contents: read
  id-token: write

env:
  DOTNET_NOLOGO: true
  DOTNET_CLI_TELEMETRY_OPTOUT: true

jobs:
  deploy:
    name: Pulumi up (shared)
    runs-on: ubuntu-latest
    timeout-minutes: 15
    environment: production
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-dotnet@v4
        with:
          global-json-file: api/global.json

      - name: Cache NuGet packages
        uses: actions/cache@v4
        with:
          path: ~/.nuget/packages
          key: nuget-infra-${{ runner.os }}-${{ hashFiles('infra/**/*.csproj') }}
          restore-keys: nuget-infra-${{ runner.os }}-

      - name: Login to Azure
        uses: azure/login@v2
        with:
          client-id: ${{ secrets.AZURE_CLIENT_ID }}
          tenant-id: ${{ secrets.AZURE_TENANT_ID }}
          subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}

      - name: Pulumi up
        uses: pulumi/actions@v6
        with:
          command: up
          stack-name: shared
          work-dir: infra
        env:
          PULUMI_ACCESS_TOKEN: ${{ secrets.PULUMI_ACCESS_TOKEN }}
          ARM_USE_OIDC: true
          ARM_CLIENT_ID: ${{ secrets.AZURE_CLIENT_ID }}
          ARM_TENANT_ID: ${{ secrets.AZURE_TENANT_ID }}
          ARM_SUBSCRIPTION_ID: ${{ secrets.AZURE_SUBSCRIPTION_ID }}
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/infra-shared-ci.yml
git commit -m "ci: add shared infrastructure deployment workflow"
```

---

### Task 6: Update `infra-ci.yml` to depend on shared workflow

**Files:**
- Modify: `.github/workflows/infra-ci.yml`

- [ ] **Step 1: Change trigger to `workflow_run`**

Replace the `on:` block so the dev infra deploy waits for the shared deploy to complete:

```yaml
on:
  workflow_run:
    workflows: ["Infrastructure Shared CI"]
    types: [completed]
    branches: [main]
```

- [ ] **Step 2: Update the deploy job condition**

The deploy job currently checks `github.event_name == 'push'`. With `workflow_run`, the event name changes. Update:

```yaml
  deploy:
    name: Pulumi up
    if: github.event.workflow_run.conclusion == 'success'
```

- [ ] **Step 3: Remove the preview job**

The `preview` job in `infra-ci.yml` triggers on `pull_request`, but this workflow no longer triggers on PRs (it uses `workflow_run`). PR previews are already handled by `pr-gate.yml`'s `infra-preview` job. Remove the entire `preview` job block (lines 33-71).

- [ ] **Step 4: Update `pr-gate.yml` path detection**

In `.github/workflows/pr-gate.yml`, update the infra path detection (line 49) to also match the new shared workflow:

```bash
infra/*|.github/workflows/infra-ci.yml|.github/workflows/infra-shared-ci.yml) has_infra=true ;;
```

- [ ] **Step 5: Commit**

```bash
git add .github/workflows/infra-ci.yml .github/workflows/pr-gate.yml
git commit -m "ci: chain dev infra deploy after shared stack via workflow_run"
```

---

### Task 7: Update `cd-prod.yml` for image promotion

**Files:**
- Modify: `.github/workflows/cd-prod.yml`

- [ ] **Step 1: Remove `acr-login-server` from infra job**

The infra job no longer needs to export `acr-login-server` — the prod stack doesn't own an ACR. Remove:
- The `acr-login-server` line from the `outputs:` block (line 40)
- The `echo "acr-login-server=..."` line from the "Extract Pulumi outputs" step (line 81)

Keep `resource-group` and `swa-name`.

- [ ] **Step 2: Replace the API job with image promotion**

Replace the entire `api` job with:

```yaml
  api:
    name: Deploy API
    needs: infra
    runs-on: ubuntu-latest
    timeout-minutes: 5
    environment: production
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Resolve tag to commit SHA
        id: resolve
        run: echo "sha=$(git rev-parse "$GITHUB_REF_NAME")" >> "$GITHUB_OUTPUT"

      - name: Login to Azure
        uses: azure/login@v2
        with:
          client-id: ${{ secrets.AZURE_CLIENT_ID }}
          tenant-id: ${{ secrets.AZURE_TENANT_ID }}
          subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}

      - name: Promote image to prod
        run: |
          az containerapp update \
            --name "ca-town-crier-api-prod" \
            --resource-group "$RESOURCE_GROUP" \
            --image "${{ secrets.ACR_LOGIN_SERVER }}/town-crier-api:${{ steps.resolve.outputs.sha }}"
        env:
          RESOURCE_GROUP: ${{ needs.infra.outputs.resource-group }}
```

Key changes from the old API job:
- No ACR login (workflow doesn't interact with registry)
- No Docker build or push
- Resolves the version tag to a commit SHA
- References the existing image by SHA in the shared ACR via `secrets.ACR_LOGIN_SERVER`
- Timeout reduced from 10 to 5 minutes

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/cd-prod.yml
git commit -m "ci: switch prod API deploy to image promotion by SHA"
```

---

### Task 8: Deploy prod stack and verify end-to-end

**Files:** None (CLI operations only)

> **WARNING:** Steps 1-2 (local prod deployment) MUST complete before Step 3 (pushing the PR). The merged code will destroy the prod ACR when `cd-prod.yml` next triggers. If the prod stack hasn't been updated to use the shared ACR + managed identity first, production will break.

- [ ] **Step 1: Preview prod stack**

Run: `cd /Users/christy/Dev/town-crier/infra && pulumi preview --stack prod --diff`
Expected: Plan to delete old ACR, update Container App with managed identity. Review carefully.

- [ ] **Step 2: Deploy prod stack**

Run: `cd /Users/christy/Dev/town-crier/infra && pulumi up --stack prod --yes`
Expected: Old ACR destroyed, Container App updated

- [ ] **Step 3: Push all changes via PR**

Create a feature branch, push, and open a PR with all commits from tasks 1-7.

- [ ] **Step 4: After merge, verify with a version tag**

Tag the merged commit and push:
```bash
git tag v0.2.0
git push origin v0.2.0
```

Verify:
- `cd-prod.yml` triggers
- Infra job succeeds
- API job promotes image by SHA (no Docker build)
- Web job deploys
- The Container App is running the promoted image

Run: `gh run watch --repo AmyDe/town-crier`
