# Shared Cosmos DB Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the Cosmos DB serverless account from per-environment stacks to the shared stack so both dev and prod can use a single account.

**Architecture:** The `DatabaseAccount` resource moves to `SharedStack.cs` and exposes account name and endpoint as outputs. `EnvironmentStack.cs` creates its database and containers in the shared account using `StackReference` outputs, the same pattern used for ACR and the Container Apps Environment.

**Tech Stack:** Pulumi (C#/.NET 10), Azure Cosmos DB (Serverless), Azure Native provider

**Spec:** `docs/superpowers/specs/2026-03-22-shared-cosmos-db-design.md`

---

### Task 1: Add Cosmos DB account to SharedStack

**Files:**
- Modify: `infra/SharedStack.cs`

- [ ] **Step 1: Add CosmosDB using directives**

Add at the top of `SharedStack.cs`, after the existing usings:

```csharp
using Pulumi.AzureNative.CosmosDB;
using Pulumi.AzureNative.CosmosDB.Inputs;
```

- [ ] **Step 2: Add DatabaseAccount resource**

Add after the `containerAppsEnv` resource (before the `return` statement):

```csharp
// Cosmos DB Account (shared across environments — serverless)
var cosmosAccount = new DatabaseAccount("cosmos-town-crier-shared", new DatabaseAccountArgs
{
    AccountName = "cosmos-town-crier-shared",
    ResourceGroupName = resourceGroup.Name,
    Kind = DatabaseAccountKind.GlobalDocumentDB,
    DatabaseAccountOfferType = DatabaseAccountOfferType.Standard,
    Capabilities = new[]
    {
        new CapabilityArgs { Name = "EnableServerless" },
    },
    ConsistencyPolicy = new ConsistencyPolicyArgs
    {
        DefaultConsistencyLevel = DefaultConsistencyLevel.Session,
    },
    Locations = new[]
    {
        new LocationArgs
        {
            LocationName = resourceGroup.Location,
            FailoverPriority = 0,
        },
    },
    Tags = tags,
});
```

- [ ] **Step 3: Add new stack outputs**

Add to the return dictionary:

```csharp
["cosmosAccountName"] = cosmosAccount.Name,
["cosmosAccountEndpoint"] = cosmosAccount.DocumentEndpoint,
```

- [ ] **Step 4: Build to verify**

Run: `dotnet build infra/`
Expected: Build succeeds with no errors.

- [ ] **Step 5: Commit**

```bash
git add infra/SharedStack.cs
git commit -m "feat(infra): add shared Cosmos DB serverless account to SharedStack"
```

---

### Task 2: Refactor EnvironmentStack to use shared Cosmos account

**Files:**
- Modify: `infra/EnvironmentStack.cs`

- [ ] **Step 1: Add shared Cosmos stack references and simplify sharedResourceGroupName**

After the existing `containerAppsEnvironmentId` shared output line (line 27), add:

```csharp
var cosmosAccountName = shared.GetOutput("cosmosAccountName").Apply(o => o?.ToString() ?? "");
var cosmosAccountEndpoint = shared.GetOutput("cosmosAccountEndpoint").Apply(o => o?.ToString() ?? "");
```

Then replace the existing `sharedResourceGroupName` derivation (lines 36-41) that parses the CAE resource ID:

```csharp
var sharedResourceGroupName = containerAppsEnvironmentId.Apply(id =>
{
    var segments = id.Split('/');
    var rgIndex = Array.IndexOf(segments, "resourceGroups");
    return rgIndex >= 0 && rgIndex + 1 < segments.Length ? segments[rgIndex + 1] : "";
});
```

With a direct stack reference:

```csharp
var sharedResourceGroupName = shared.GetOutput("resourceGroupName").Apply(o => o?.ToString() ?? "");
```

This is clearer and less fragile than parsing the resource group name from the CAE resource ID.

- [ ] **Step 2: Remove Cosmos config reads and dead code**

Remove these lines from the top of the `Run` method:

```csharp
var cosmosConsistencyLevel = config.Require("cosmosConsistencyLevel");
```
```csharp
var importExistingCosmos = config.GetBoolean("importExistingCosmos") == true;
```
```csharp
var skipCosmosDb = config.GetBoolean("skipCosmosDb") == true;
```

Remove the entire `cosmosImportOpts` block:

```csharp
var cosmosImportOpts = importExistingCosmos
    ? new CustomResourceOptions
    {
        ImportId = $"/subscriptions/{Environment.GetEnvironmentVariable("ARM_SUBSCRIPTION_ID")}/resourceGroups/rg-town-crier-{env}/providers/Microsoft.DocumentDB/databaseAccounts/cosmos-town-crier-{env}",
    }
    : null;
```

- [ ] **Step 3: Remove DatabaseAccount creation and replace with shared references**

> **Note:** Steps 3–6 restructure the Cosmos section as a unit. The code won't compile until Step 6 is complete. Apply them together before building.

Remove the `DatabaseAccount? cosmosAccount = null;` and `SqlResourceSqlDatabase? cosmosDatabase = null;` declarations.

Remove the `if (!skipCosmosDb)` guard and its opening brace. The `DatabaseAccount` creation block (the `new DatabaseAccount(...)` call, lines 64–95) is deleted entirely.

Keep the database and container creation code but **un-indent it** (it's no longer inside an `if` block).

- [ ] **Step 4: Update database creation to use shared account**

Replace the database creation with:

```csharp
// Cosmos DB Database (in shared account)
var cosmosDatabase = new SqlResourceSqlDatabase($"db-town-crier-{env}", new SqlResourceSqlDatabaseArgs
{
    AccountName = cosmosAccountName,
    ResourceGroupName = sharedResourceGroupName,
    DatabaseName = $"town-crier-{env}",
    Resource = new SqlDatabaseResourceArgs
    {
        Id = $"town-crier-{env}",
    },
});
```

Key changes from the original:
- `AccountName` uses `cosmosAccountName` (shared) instead of `cosmosAccount.Name` (local)
- `ResourceGroupName` uses `sharedResourceGroupName` instead of `resourceGroup.Name`
- `DatabaseName` and `Resource.Id` change from `"town-crier"` to `$"town-crier-{env}"`

- [ ] **Step 5: Update all container resources**

For each of the 5 containers (Applications, Users, WatchZones, Notifications, Leases), make these two substitutions:

- `AccountName = cosmosAccount.Name,` → `AccountName = cosmosAccountName,`
- `ResourceGroupName = resourceGroup.Name,` → `ResourceGroupName = sharedResourceGroupName,`

The containers reference `cosmosDatabase.Name` for `DatabaseName` — this stays the same (it will now resolve to `town-crier-{env}`).

- [ ] **Step 6: Remove the closing brace of the old `if (!skipCosmosDb)` block**

The closing `}` on what was line 258 is no longer needed since we removed the `if` guard.

- [ ] **Step 7: Update stack outputs**

Replace:

```csharp
["cosmosAccountEndpoint"] = cosmosAccount?.DocumentEndpoint,
["cosmosDatabaseName"] = cosmosDatabase?.Name,
```

With:

```csharp
["cosmosAccountEndpoint"] = cosmosAccountEndpoint,
["cosmosDatabaseName"] = cosmosDatabase.Name,
```

No more nullable — Cosmos is always created now.

- [ ] **Step 8: Clean up unused usings if needed**

The `CosmosDB` usings should stay — they're still needed for `SqlResourceSqlDatabase`, `SqlResourceSqlContainer`, etc. But `DatabaseAccount`, `DatabaseAccountArgs`, `DatabaseAccountKind`, `DatabaseAccountOfferType`, `ConsistencyPolicyArgs`, `DefaultConsistencyLevel`, `CapabilityArgs`, and `LocationArgs` are no longer used in this file. The usings are namespace-level so they stay (other types from those namespaces are still used).

- [ ] **Step 9: Build to verify**

Run: `dotnet build infra/`
Expected: Build succeeds with no errors.

- [ ] **Step 10: Commit**

```bash
git add infra/EnvironmentStack.cs
git commit -m "refactor(infra): use shared Cosmos account in EnvironmentStack"
```

---

### Task 3: Update Pulumi config files

**Files:**
- Modify: `infra/Pulumi.dev.yaml`
- Modify: `infra/Pulumi.prod.yaml`

- [ ] **Step 1: Remove cosmosConsistencyLevel from dev config**

Edit `infra/Pulumi.dev.yaml` — remove the line:

```yaml
  town-crier:cosmosConsistencyLevel: Session
```

Final contents:

```yaml
config:
  azure-native:location: uksouth
  town-crier:environment: dev
  town-crier:frontendDomain: dev.towncrierapp.uk
  town-crier:apiDomain: api-dev.towncrierapp.uk
```

- [ ] **Step 2: Remove cosmosConsistencyLevel from prod config**

Edit `infra/Pulumi.prod.yaml` — remove the line:

```yaml
  town-crier:cosmosConsistencyLevel: Session
```

Final contents:

```yaml
config:
  azure-native:location: uksouth
  town-crier:environment: prod
  town-crier:frontendDomain: towncrierapp.uk
  town-crier:apiDomain: api.towncrierapp.uk
  town-crier:customDomainPhase: "2"
```

- [ ] **Step 3: Build to verify configs don't break anything**

Run: `dotnet build infra/`
Expected: Build succeeds (config is read at runtime, not build time, but this catches any compilation issues from previous tasks).

- [ ] **Step 4: Commit**

```bash
git add infra/Pulumi.dev.yaml infra/Pulumi.prod.yaml
git commit -m "chore(infra): remove cosmosConsistencyLevel from env configs"
```

---

### Task 4: Ship and deploy

This task is manual — it requires CI/CD pipelines and Azure.

- [ ] **Step 1: Ship to main**

Use the ship skill to merge all commits to main via PR.

- [ ] **Step 2: Wait for cd-dev to complete**

cd-dev runs on push to main. It will:
1. Deploy **shared stack** — creates `cosmos-town-crier-shared` account
2. Deploy **dev stack** — deletes old `cosmos-town-crier-dev` account, creates `town-crier-dev` database and containers in the shared account

Run: `gh run list --workflow=cd-dev.yml --limit 1`
Watch: `gh run watch <run-id> --exit-status`

If it fails, check logs with `gh run view <run-id> --log-failed`.

- [ ] **Step 3: Tag for prod deployment**

```bash
git tag v0.2.1
git push origin v0.2.1
```

cd-prod will deploy prod stack — creates `town-crier-prod` database and containers in the shared Cosmos account.

- [ ] **Step 4: Watch cd-prod**

Run: `gh run watch <run-id> --exit-status`

- [ ] **Step 5: Validate**

```bash
# API should respond (still 500 until wired to Cosmos, but Kestrel is running)
curl -sI --max-time 30 https://api.towncrierapp.uk/health

# Frontend should be 200
curl -sI --max-time 10 https://towncrierapp.uk

# Verify Cosmos account exists
az cosmosdb list --query "[].name" -o tsv
# Expected: cosmos-town-crier-shared (and no cosmos-town-crier-dev)

# Verify both databases exist in shared account
az cosmosdb sql database list --account-name cosmos-town-crier-shared --resource-group rg-town-crier-shared --query "[].name" -o tsv
# Expected: town-crier-dev, town-crier-prod
```
