# Integration Tests Design

Date: 2026-03-30

## Problem

The full stack (web app -> Auth0 -> API -> Cosmos DB) has never worked end-to-end. Each deployment fix reveals the next issue (CORS, AOT compilation, Cosmos SDK compatibility). There is no automated way to answer "does it work yet?" after each fix.

Existing tests are unit and in-process integration tests. Nothing exercises the real deployed infrastructure with real authentication.

## Decision

Add an API-level integration test project that authenticates with the real Auth0 dev tenant and hits the deployed API. These are smoke tests — happy-path only — proving the stack is wired up, not providing coverage (that comes from lower-level tests).

Run these tests in the PR gate against a staging revision of the Azure Container App before promoting it to serve live traffic.

## Test Project

**Location:** `api/tests/town-crier.integration-tests`

**Framework:** TUnit (consistent with existing test projects)

**Configuration:** Environment variables:

| Variable | Purpose |
|----------|---------|
| `INTEGRATION_TEST_API_BASE_URL` | Staging revision URL |
| `INTEGRATION_TEST_AUTH0_DOMAIN` | Auth0 dev tenant domain |
| `INTEGRATION_TEST_AUTH0_CLIENT_ID` | Auth0 application client ID (existing SPA app) |
| `INTEGRATION_TEST_AUTH0_CLIENT_SECRET` | Auth0 client secret |
| `INTEGRATION_TEST_AUTH0_AUDIENCE` | API audience identifier |
| `INTEGRATION_TEST_USERNAME` | Test user email |
| `INTEGRATION_TEST_PASSWORD` | Test user password |

**Auth0 token acquisition:** Resource Owner Password Grant, executed once per test run in a shared fixture. The token is reused across all tests.

**Auth0 application:** Reuse the existing Auth0 application. Enable the Password grant type on it.

## Test Cases

Three happy-path smoke tests, run in order:

### 1. Health check

- `GET /v1/health` -> 200 OK
- No auth required
- Proves the revision is running and reachable

### 2. Create and retrieve user profile

- `POST /api/me/profile` with basic profile data -> 201 or 200
- `GET /api/me/profile` -> 200, response contains submitted data
- Proves: Auth0 JWT accepted, user identity extracted from token, Cosmos DB write/read works, serialization is AOT-compatible

### 3. Create and retrieve watch zone

- `POST /api/me/watch-zones` with zone data (name, postcode, radius) -> 201
- `GET /api/me/watch-zones` -> 200, response contains the created zone
- Proves: full domain feature works end-to-end

No cleanup step. The test user accumulates data in dev Cosmos, which is acceptable.

## CI Pipeline — PR Gate

New job in `pr-gate.yml` that runs after the API build:

```
1. Build API Docker image (reuse existing build step)
2. Push image to Azure Container Registry
3. Deploy as new revision to dev Container App
   - Revision suffix: pr-<number>
   - Traffic weight: 0% (latest=false)
4. Retrieve revision-specific FQDN
5. Run: dotnet test town-crier.integration-tests
6. On success: shift 100% traffic to new revision
7. On failure: deactivate staging revision, fail the PR check
```

### Secrets Required in GitHub Actions

- `INTEGRATION_TEST_USERNAME` — test user email
- `INTEGRATION_TEST_PASSWORD` — test user password
- Auth0 domain, client ID, client secret, and audience — from existing config or new secrets
- Azure credentials — already exist for deployment

### Change to Existing CD Flow

Today `cd-dev.yml` deploys on push to main. With this change, the PR gate deploys and promotes the staging revision. The cd-dev deploy may become redundant, or can remain as a fallback for direct pushes to main.

## Infrastructure Changes

### Azure Container Apps — Multi-Revision Mode

The dev Container App must be switched from single-revision mode to multi-revision mode. This allows multiple revisions to coexist with independent URLs and traffic weights.

**Change:** Set `activeRevisionsMode` to `Multiple` in the Pulumi Container App resource for the dev stack.

## Auth0 Setup (Manual, One-Time)

1. Create a test user in the Auth0 dev tenant (e.g. `integration-test@towncrierapp.uk`) with a known password
2. Enable the "Password" grant type on the existing Auth0 application (Dashboard -> Applications -> Settings -> Advanced -> Grant Types)
3. Store the test user credentials as GitHub Actions secrets

## Consequences

**What becomes easier:**
- Every PR automatically proves the API works against real infrastructure before merging
- Deployment issues (AOT, Cosmos, auth) are caught before they reach the live dev environment
- The "does it work yet?" question is answered automatically

**What becomes harder/riskier:**
- PRs take longer due to the deploy-and-test cycle
- The dev environment is shared — concurrent PRs could conflict (acceptable for a solo dev)
- Resource Owner Password Grant is considered legacy by Auth0 — may need to revisit if Auth0 deprecates it
- Multi-revision mode means old revisions need occasional cleanup
