# 0015. CI/CD Pipeline and Deployment Strategy

Date: 2026-03-28

## Status

Accepted

## Context

Town Crier is a monorepo with four independently deployable components: a .NET API, an iOS app, a React web frontend, and Pulumi infrastructure. Each component has different build tools, test suites, and deployment targets. The CI/CD pipeline needs to validate changes efficiently (avoiding full rebuilds when only one component changes), deploy to multiple environments safely, and authenticate to Azure without storing long-lived secrets.

ADR 0001 selected GitHub Actions for CI/CD but did not specify the pipeline architecture, environment promotion strategy, or authentication model.

## Decision

### Pipeline Architecture: Three Workflows

1. **PR Gate** (`pr-gate.yml`) — runs on every pull request to `main`. Uses path-based change detection to determine which components changed (`api/`, `mobile/ios/`, `web/`, `infra/`) and only runs the relevant quality gates. A single required status check (`gate`) aggregates all component results, so GitHub branch protection needs only one check regardless of which components ran.

2. **CD Dev** (`cd-dev.yml`) — runs on every push to `main` (i.e., after PR merge). Deploys changed components to the development environment automatically. Infrastructure stacks (`shared` then `dev`) are applied first, then API and web are deployed in parallel.

3. **CD Prod** (`cd-prod.yml`) — runs on semver tags matching `v*`. Deploys to the production environment. The API image is resolved by the tag's commit SHA (falling back to `:latest` if the SHA-tagged image is unavailable). This ensures production always runs the exact code that was tagged.

### Component-Aware Change Detection

Each workflow detects changes per component directory. If only `/web` changed, only web quality gates and web deployment run. This keeps PR feedback fast and avoids unnecessary Azure resource churn. Shared workflow file changes trigger all component checks.

### Quality Gates (PR)

| Component | Checks |
|-----------|--------|
| API | `dotnet format --verify-no-changes`, `dotnet build` (Release), `dotnet test` |
| iOS | SwiftLint (`--strict`), `xcodebuild` (iPhone 16 simulator), `swift test` |
| Web | ESLint, TypeScript type-check (`tsc --noEmit`), Vitest, Vite production build |
| Infra | `pulumi preview` with PR comment showing planned changes |

### Azure Authentication: OIDC Federated Credentials

All workflows authenticate to Azure using **OpenID Connect (OIDC) federated credentials** rather than stored service principal secrets. GitHub Actions requests a short-lived token from Azure AD using the workflow's OIDC identity, scoped to the specific environment (development or production). This eliminates secret rotation, reduces blast radius of credential compromise, and aligns with zero-trust principles.

Secrets stored in GitHub:
- `AZURE_CLIENT_ID`, `AZURE_TENANT_ID`, `AZURE_SUBSCRIPTION_ID` — identity coordinates (not sensitive on their own)
- `ACR_LOGIN_SERVER` — container registry hostname
- `PULUMI_ACCESS_TOKEN` — Pulumi Cloud state backend authentication

No Azure service principal passwords or client secrets are stored.

### Environment Promotion Strategy

| Trigger | Target | Mechanism |
|---------|--------|-----------|
| PR opened/updated | Preview | Pulumi preview (comment on PR), web type-check and tests |
| PR merged to `main` | Development | Automatic: infra up, API image build + deploy, web SWA deploy |
| Semver tag (`v*`) | Production | Automatic: infra up, API image resolve + deploy, web SWA deploy |

Production deployments require an explicit tagging action — they never happen automatically on merge. This provides a deliberate release gate while still keeping the deployment itself automated.

### Container Image Strategy

The API is built as a Docker image and pushed to Azure Container Registry with two tags: the commit SHA (`town-crier-api:{sha}`) and `:latest`. The dev CD pipeline always builds and pushes a fresh image. The prod CD pipeline resolves the tag's commit SHA and pulls the corresponding pre-built image from ACR, ensuring prod runs the exact same binary that was tested in dev.

### Web Deployment

The React SPA is deployed to **Azure Static Web Apps** using the official `Azure/static-web-apps-deploy` GitHub Action. Environment-specific configuration (API base URL, Auth0 domain/client ID/audience) is injected at build time via `VITE_*` environment variables stored as GitHub Actions variables.

## Consequences

- **Faster PR feedback** — component-aware detection means a web-only change doesn't wait for .NET builds or iOS simulator tests.
- **No secret rotation burden** — OIDC federated credentials are ephemeral. No periodic rotation, no leaked-secret incident response.
- **Deterministic production deploys** — SHA-tagged images guarantee prod runs the tested code, not whatever `:latest` happens to be.
- **Tag-based release gating** — production deploys require human intent (creating a tag), preventing accidental releases from fast-forwarded merges.
- **Single required check simplifies branch protection** — the `gate` job aggregates all components, so adding a new component doesn't require updating GitHub branch protection rules.
- **iOS builds run on macOS runners** — GitHub's macOS runners are slower and more expensive than Linux. The component-aware detection mitigates this by only running iOS checks when `/mobile/ios` changes.
