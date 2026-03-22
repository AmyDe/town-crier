# Shared Cosmos DB Account

Date: 2026-03-22

## Problem

Azure limits serverless Cosmos DB accounts per subscription per region. The dev environment already has a serverless account in UK South, so creating a second one for prod fails with `ServiceUnavailable` ("high demand in UK West region"). This blocks prod deployment.

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

**Modify:**
- Database name changes from `town-crier` to `town-crier-{env}` (both envs share one account, so databases must have distinct names)
- All container creation stays identical, just references `cosmosAccountName` from shared instead of a locally-created account
- `cosmosAccountEndpoint` replaces the local `cosmosAccount.DocumentEndpoint` in stack outputs

### Config changes

| File | Change |
|------|--------|
| `Pulumi.shared.yaml` | No changes |
| `Pulumi.dev.yaml` | Remove `cosmosConsistencyLevel` |
| `Pulumi.prod.yaml` | Remove `cosmosConsistencyLevel` |

### Pulumi state

The existing `cosmos-town-crier-dev` account in the dev stack state needs to be removed. Since there is no data worth preserving, we can either:
- Let `pulumi up` on the dev stack destroy it (it will see the `DatabaseAccount` resource removed from the code and delete it)
- The shared stack then creates the new `cosmos-town-crier-shared` account

The dev stack deploy must run after the shared stack deploy, which is already the case in cd-dev.yml (shared runs first, dev depends on it).

### Deployment order

1. **Shared stack** deploys first (creates the new Cosmos account)
2. **Dev stack** deploys second (old Cosmos account gets deleted by Pulumi since it's no longer in code; new database and containers created using shared account)
3. **Prod stack** deploys via tag (creates its database and containers using the shared account)

cd-dev.yml already enforces shared-before-dev ordering. cd-prod.yml does not deploy the shared stack, but it doesn't need to — the shared stack will already have the Cosmos account after the next cd-dev run.

### What doesn't change

- Container definitions (Applications, Users, WatchZones, Notifications, Leases) and their partition keys, indexes, TTLs, unique constraints
- Container App, Static Web App, managed certificate resources
- CI/CD workflow files
- API application code (connection details come from environment variables at runtime)

## Consequences

**Easier:** Prod deployment unblocked. No subscription-level Cosmos limits to worry about. One fewer resource to manage.

**Harder:** Shared account becomes a single point of failure for both environments. Account-level settings (consistency, region) can't differ between envs. If the account is accidentally deleted, both envs lose their database. These trade-offs are acceptable for a solo project with no users; when that changes, split back into per-env accounts.
