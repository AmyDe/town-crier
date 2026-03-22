# Shared Cosmos DB Account

Date: 2026-03-22

## Problem

Azure limits serverless Cosmos DB accounts per subscription per region. The dev environment already has a serverless account in UK South, so creating a second one for prod fails with `ServiceUnavailable`. The error references "UK West region" despite the config targeting `uksouth` — Azure internally routes serverless provisioning across paired regions. This blocks prod deployment.

## Decision

Move the Cosmos DB serverless account from the per-environment stacks into the shared stack. Both dev and prod create their own databases and containers within the single shared account. This mirrors the existing pattern for ACR and the Container Apps Environment.

## Design

### SharedStack

Add a `DatabaseAccount` resource:

- **Name:** `cosmos-town-crier-shared`
- **Kind:** `GlobalDocumentDB`
- **Offer:** `Standard` with `EnableServerless` capability
- **Consistency:** `Session` (hardcoded; both envs use Session)
- **Location:** Inherits from `azure-native:location` (UK South)

New stack outputs:
- `cosmosAccountName` — account name string
- `cosmosAccountEndpoint` — document endpoint URL

No config changes to `Pulumi.shared.yaml` required.

### EnvironmentStack

**Remove:**
- `DatabaseAccount` creation
- `skipCosmosDb` flag and all conditional logic around it
- `importExistingCosmos` flag and import options
- `cosmosConsistencyLevel` config read

**Add:**
- `cosmosAccountName` from shared stack via `StackReference` (same pattern as `acrLoginServer`)
- `cosmosAccountEndpoint` from shared stack
- `sharedResourceGroupName` from shared stack via `StackReference` — needed because Cosmos databases and containers must be created in the same resource group as the account (`rg-town-crier-shared`), not the per-env resource group. The shared stack already exports `resourceGroupName`; the env stack currently derives it from the CAE resource ID, but should use the direct output for clarity.

**Modify:**
- Database name changes from `town-crier` to `town-crier-{env}` (both envs share one account, so databases must have distinct names)
- All `SqlResourceSqlDatabase` and `SqlResourceSqlContainer` resources change their `ResourceGroupName` from `resourceGroup.Name` (per-env) to the shared resource group name
- `cosmosAccountEndpoint` replaces the local `cosmosAccount.DocumentEndpoint` in stack outputs

**Note on database name:** The API is not yet wired to Cosmos at runtime. When it is, it must read the database name from configuration (e.g., stack output → env var) rather than hardcoding `town-crier`. The stack already exports `cosmosDatabaseName` for this purpose.

### Config changes

| File | Change |
|------|--------|
| `Pulumi.shared.yaml` | No changes |
| `Pulumi.dev.yaml` | Remove `cosmosConsistencyLevel` |
| `Pulumi.prod.yaml` | Remove `cosmosConsistencyLevel` |

### Pulumi state

**Dev stack:** The existing `cosmos-town-crier-dev` account and its databases/containers are in the dev stack state. Since there is no data worth preserving, Pulumi will delete them when it sees the resources removed from code. The shared stack then creates the new `cosmos-town-crier-shared` account.

**Prod stack:** Cosmos was never successfully provisioned in prod (`skipCosmosDb` was always `true`), so there are no Cosmos resources in the prod Pulumi state. No state cleanup needed.

### Deployment order

1. **Shared stack** deploys first (creates the new Cosmos account)
2. **Dev stack** deploys second (old Cosmos account deleted by Pulumi; new database and containers created using shared account)
3. **Prod stack** deploys via tag (creates its database and containers using the shared account)

cd-dev.yml already enforces shared-before-dev ordering. cd-prod.yml does not deploy the shared stack — the shared stack must have been deployed (via a cd-dev run) before a prod tag is cut. This is safe because any push to main that includes infra changes will trigger cd-dev first.

### What doesn't change

- Container definitions (Applications, Users, WatchZones, Notifications, Leases) and their partition keys, indexes, TTLs, unique constraints
- Container App, Static Web App, managed certificate resources
- CI/CD workflow files (cd-dev.yml already deploys shared before dev; cd-prod.yml doesn't need shared since it will exist)
- API application code (not yet wired to Cosmos; when it is, it reads connection details from env vars)

## Consequences

**Easier:** Prod deployment unblocked. No subscription-level Cosmos limits to worry about. One fewer resource to manage.

**Harder:** Shared account becomes a single point of failure for both environments. Account-level settings (consistency, region) can't differ between envs. If the account is accidentally deleted, both envs lose their database. These trade-offs are acceptable for a solo project with no users; when that changes, split back into per-env accounts.
