# CI/CD Pipeline Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Consolidate 7 GitHub Actions workflows into 3: PR gate (test-only), CD dev (deploy on main push), CD prod (deploy on tag push).

**Architecture:** Path-filtered PR gate runs tests/lint/format. On main merge, cd-dev builds the API Docker image and deploys all changed components to the dev environment. On version tag, cd-prod promotes the existing API image and deploys web to production.

**Tech Stack:** GitHub Actions, Azure OIDC, Pulumi, Docker/ACR, Azure Container Apps, Azure Static Web Apps

**Spec:** `docs/superpowers/specs/2026-03-22-cicd-pipeline-redesign.md`

---

### Task 1: Update pr-gate.yml path patterns

**Files:**
- Modify: `.github/workflows/pr-gate.yml:44-51` (path detection case statement)

The existing path patterns reference workflow files that will be deleted. Update them now so the PR gate still works after the old files are removed.

- [ ] **Step 1: Update the case statement path patterns**

In `.github/workflows/pr-gate.yml`, replace the path-detection script (lines 34-58). The key change is that shared workflow files (`cd-dev.yml`, `pr-gate.yml`, `cd-prod.yml`) appear in multiple component categories, so they must be handled outside the `case` statement (bash `case` uses first-match semantics and would only trigger the first matching branch):

```yaml
        run: |
          CHANGED=$(git diff --name-only origin/${{ github.base_ref }}...HEAD)
          echo "Changed files:"
          echo "$CHANGED"

          has_api=false
          has_ios=false
          has_web=false
          has_infra=false

          # Shared workflow files trigger multiple components — check before per-file loop
          if echo "$CHANGED" | grep -q '.github/workflows/pr-gate.yml'; then
            has_api=true; has_ios=true; has_web=true; has_infra=true
          fi
          if echo "$CHANGED" | grep -q '.github/workflows/cd-dev.yml'; then
            has_api=true; has_web=true; has_infra=true
          fi
          if echo "$CHANGED" | grep -q '.github/workflows/cd-prod.yml'; then
            has_infra=true
          fi

          while IFS= read -r file; do
            case "$file" in
              api/*) has_api=true ;;
              mobile/ios/*) has_ios=true ;;
              web/*) has_web=true ;;
              infra/*) has_infra=true ;;
            esac
          done <<< "$CHANGED"

          echo "api=$has_api" >> "$GITHUB_OUTPUT"
          echo "ios=$has_ios" >> "$GITHUB_OUTPUT"
          echo "web=$has_web" >> "$GITHUB_OUTPUT"
          echo "infra=$has_infra" >> "$GITHUB_OUTPUT"

          echo "Results: api=$has_api ios=$has_ios web=$has_web infra=$has_infra"
```

This replaces references to `api-ci.yml`, `web-ci.yml`, `ios-ci.yml`, `infra-ci.yml`, and `infra-shared-ci.yml` with the new workflow file names, and handles shared files correctly by checking them before the per-file loop.

- [ ] **Step 2: Verify the YAML is valid**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/pr-gate.yml'))"`
Expected: No output (valid YAML)

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/pr-gate.yml
git commit -m "ci: update pr-gate path patterns for new workflow names"
```

---

### Task 2: Create cd-dev.yml

**Files:**
- Create: `.github/workflows/cd-dev.yml`

This is the core new workflow. It triggers on push to main, detects which components changed, and deploys them to the dev environment.

- [ ] **Step 1: Create the workflow file**

Create `.github/workflows/cd-dev.yml` with this content:

```yaml
# Deploys changed components to the dev environment.
# Triggered on every push to main (squash merges assumed).
#
# Requires secrets:
#   AZURE_CLIENT_ID        — OIDC federated credential for Azure login
#   AZURE_TENANT_ID        — Azure AD tenant
#   AZURE_SUBSCRIPTION_ID  — Azure subscription
#   ACR_LOGIN_SERVER       — Azure Container Registry login server
#   PULUMI_ACCESS_TOKEN    — Pulumi Cloud access token
#
# Requires GitHub Environment:
#   development            — with OIDC federated credential (environment:development)

name: CD Dev

on:
  push:
    branches: [main]

concurrency:
  group: cd-dev
  cancel-in-progress: false

permissions:
  contents: read
  id-token: write

env:
  DOTNET_NOLOGO: true
  DOTNET_CLI_TELEMETRY_OPTOUT: true

jobs:
  # ── Change detection ─────────────────────────────────
  changes:
    name: Detect changes
    runs-on: ubuntu-latest
    timeout-minutes: 2
    outputs:
      api: ${{ steps.filter.outputs.api }}
      web: ${{ steps.filter.outputs.web }}
      infra: ${{ steps.filter.outputs.infra }}
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 2
      - name: Detect changed paths
        id: filter
        run: |
          CHANGED=$(git diff --name-only HEAD~1 HEAD)
          echo "Changed files:"
          echo "$CHANGED"

          has_api=false
          has_web=false
          has_infra=false

          # cd-dev.yml triggers all components — check before per-file loop
          if echo "$CHANGED" | grep -q '.github/workflows/cd-dev.yml'; then
            has_api=true; has_web=true; has_infra=true
          fi

          while IFS= read -r file; do
            case "$file" in
              api/*) has_api=true ;;
              web/*) has_web=true ;;
              infra/*) has_infra=true ;;
            esac
          done <<< "$CHANGED"

          echo "api=$has_api" >> "$GITHUB_OUTPUT"
          echo "web=$has_web" >> "$GITHUB_OUTPUT"
          echo "infra=$has_infra" >> "$GITHUB_OUTPUT"

          echo "Results: api=$has_api web=$has_web infra=$has_infra"

  # ── Infrastructure: shared stack ─────────────────────
  infra-shared:
    name: "Infra: Pulumi up (shared)"
    needs: changes
    if: needs.changes.outputs.infra == 'true'
    runs-on: ubuntu-latest
    timeout-minutes: 15
    environment: development
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-dotnet@v4
        with:
          global-json-file: api/global.json

      - uses: actions/cache@v4
        with:
          path: ~/.nuget/packages
          key: nuget-infra-${{ runner.os }}-${{ hashFiles('infra/**/*.csproj') }}
          restore-keys: nuget-infra-${{ runner.os }}-

      - uses: azure/login@v2
        with:
          client-id: ${{ secrets.AZURE_CLIENT_ID }}
          tenant-id: ${{ secrets.AZURE_TENANT_ID }}
          subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}

      - uses: pulumi/actions@v6
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

  # ── Infrastructure: dev stack ────────────────────────
  infra-dev:
    name: "Infra: Pulumi up (dev)"
    needs: [changes, infra-shared]
    if: needs.changes.outputs.infra == 'true'
    runs-on: ubuntu-latest
    timeout-minutes: 15
    environment: development
    outputs:
      resource-group: ${{ steps.outputs.outputs.resource-group }}
      swa-name: ${{ steps.outputs.outputs.swa-name }}
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-dotnet@v4
        with:
          global-json-file: api/global.json

      - uses: actions/cache@v4
        with:
          path: ~/.nuget/packages
          key: nuget-infra-${{ runner.os }}-${{ hashFiles('infra/**/*.csproj') }}
          restore-keys: nuget-infra-${{ runner.os }}-

      - uses: azure/login@v2
        with:
          client-id: ${{ secrets.AZURE_CLIENT_ID }}
          tenant-id: ${{ secrets.AZURE_TENANT_ID }}
          subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}

      - uses: pulumi/actions@v6
        with:
          command: up
          stack-name: dev
          work-dir: infra
        env:
          PULUMI_ACCESS_TOKEN: ${{ secrets.PULUMI_ACCESS_TOKEN }}
          ARM_USE_OIDC: true
          ARM_CLIENT_ID: ${{ secrets.AZURE_CLIENT_ID }}
          ARM_TENANT_ID: ${{ secrets.AZURE_TENANT_ID }}
          ARM_SUBSCRIPTION_ID: ${{ secrets.AZURE_SUBSCRIPTION_ID }}

      - name: Extract Pulumi outputs
        id: outputs
        working-directory: infra
        run: |
          echo "resource-group=$(pulumi stack output resourceGroupName --stack dev)" >> "$GITHUB_OUTPUT"
          echo "swa-name=$(pulumi stack output staticWebAppName --stack dev)" >> "$GITHUB_OUTPUT"
        env:
          PULUMI_ACCESS_TOKEN: ${{ secrets.PULUMI_ACCESS_TOKEN }}

  # ── API: build & push image ─────────────────────────
  api-image:
    name: "API: Build & push image"
    needs: changes
    if: needs.changes.outputs.api == 'true'
    runs-on: ubuntu-latest
    timeout-minutes: 10
    environment: development
    steps:
      - uses: actions/checkout@v4

      - uses: azure/login@v2
        with:
          client-id: ${{ secrets.AZURE_CLIENT_ID }}
          tenant-id: ${{ secrets.AZURE_TENANT_ID }}
          subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}

      - name: Login to ACR
        run: az acr login --name ${{ secrets.ACR_LOGIN_SERVER }}

      - name: Build and push image
        run: |
          IMAGE="${{ secrets.ACR_LOGIN_SERVER }}/town-crier-api:${{ github.sha }}"
          docker build -t "$IMAGE" -t "${{ secrets.ACR_LOGIN_SERVER }}/town-crier-api:latest" .
          docker push "$IMAGE"
          docker push "${{ secrets.ACR_LOGIN_SERVER }}/town-crier-api:latest"
        working-directory: api

  # ── API: deploy to dev ──────────────────────────────
  api-deploy:
    name: "API: Deploy to dev"
    needs: [changes, api-image, infra-dev]
    if: >-
      !failure() && !cancelled()
      && needs.changes.outputs.api == 'true'
    runs-on: ubuntu-latest
    timeout-minutes: 5
    environment: development
    steps:
      - uses: azure/login@v2
        with:
          client-id: ${{ secrets.AZURE_CLIENT_ID }}
          tenant-id: ${{ secrets.AZURE_TENANT_ID }}
          subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}

      - name: Deploy to Container App
        run: |
          az containerapp update \
            --name "ca-town-crier-api-dev" \
            --resource-group "rg-town-crier-dev" \
            --image "${{ secrets.ACR_LOGIN_SERVER }}/town-crier-api:${{ github.sha }}"

  # ── Web: deploy to dev ──────────────────────────────
  web-deploy:
    name: "Web: Deploy to dev"
    needs: [changes, infra-dev]
    if: >-
      !failure() && !cancelled()
      && needs.changes.outputs.web == 'true'
    runs-on: ubuntu-latest
    timeout-minutes: 10
    environment: development
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v4
        with:
          node-version: 22
          cache: npm
          cache-dependency-path: web/package-lock.json

      - name: Install dependencies
        run: npm ci
        working-directory: web

      - name: Build
        run: npm run build
        working-directory: web

      - uses: azure/login@v2
        with:
          client-id: ${{ secrets.AZURE_CLIENT_ID }}
          tenant-id: ${{ secrets.AZURE_TENANT_ID }}
          subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}

      - name: Get SWA deployment token
        id: swa-token
        run: |
          TOKEN=$(az staticwebapp secrets list \
            --name "swa-town-crier-dev" \
            --query "properties.apiKey" -o tsv)
          echo "::add-mask::$TOKEN"
          echo "token=$TOKEN" >> "$GITHUB_OUTPUT"

      - name: Deploy to Azure Static Web Apps
        uses: Azure/static-web-apps-deploy@v1
        with:
          azure_static_web_apps_api_token: ${{ steps.swa-token.outputs.token }}
          repo_token: ${{ secrets.GITHUB_TOKEN }}
          action: upload
          app_location: web/dist
          skip_app_build: true
          skip_api_build: true
```

- [ ] **Step 2: Verify the YAML is valid**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/cd-dev.yml'))"`
Expected: No output (valid YAML)

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/cd-dev.yml
git commit -m "ci: add cd-dev workflow for dev deployment on main push"
```

---

### Task 3: Add image existence guard to cd-prod.yml

**Files:**
- Modify: `.github/workflows/cd-prod.yml:108-115` (API promote step)

Only API-changing commits produce images in the new model. The prod deploy must check if the image exists before attempting to update the Container App.

- [ ] **Step 1: Add image existence check before the promote step**

In `.github/workflows/cd-prod.yml`, replace the "Promote image to prod" step (lines 108-115) with:

```yaml
      - name: Check if API image exists
        id: check-image
        run: |
          ACR_NAME="${{ secrets.ACR_LOGIN_SERVER }}"
          ACR_NAME="${ACR_NAME%.azurecr.io}"
          if az acr manifest show \
            --registry "$ACR_NAME" \
            --name "town-crier-api:${{ steps.resolve.outputs.sha }}" 2>/dev/null; then
            echo "exists=true" >> "$GITHUB_OUTPUT"
          else
            echo "No API image found for SHA ${{ steps.resolve.outputs.sha }} — skipping API deploy"
            echo "exists=false" >> "$GITHUB_OUTPUT"
          fi

      - name: Promote image to prod
        if: steps.check-image.outputs.exists == 'true'
        run: |
          az containerapp update \
            --name "ca-town-crier-api-prod" \
            --resource-group "$RESOURCE_GROUP" \
            --image "${{ secrets.ACR_LOGIN_SERVER }}/town-crier-api:${{ steps.resolve.outputs.sha }}"
        env:
          RESOURCE_GROUP: ${{ needs.infra.outputs.resource-group }}
```

- [ ] **Step 2: Verify the YAML is valid**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/cd-prod.yml'))"`
Expected: No output (valid YAML)

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/cd-prod.yml
git commit -m "ci: guard cd-prod API deploy with ACR image existence check"
```

---

### Task 4: Delete redundant workflow files

**Files:**
- Delete: `.github/workflows/api-ci.yml`
- Delete: `.github/workflows/web-ci.yml`
- Delete: `.github/workflows/ios-ci.yml`
- Delete: `.github/workflows/infra-ci.yml`
- Delete: `.github/workflows/infra-shared-ci.yml`

These are all replaced by the combination of pr-gate.yml (tests) and cd-dev.yml (deploys).

- [ ] **Step 1: Delete the files**

```bash
rm -f .github/workflows/api-ci.yml .github/workflows/web-ci.yml .github/workflows/ios-ci.yml .github/workflows/infra-ci.yml .github/workflows/infra-shared-ci.yml
```

- [ ] **Step 2: Verify only 3 workflow files remain**

```bash
ls .github/workflows/
```

Expected output:
```
cd-dev.yml
cd-prod.yml
pr-gate.yml
```

- [ ] **Step 3: Commit**

```bash
git add -u .github/workflows/
git commit -m "ci: remove redundant workflow files replaced by cd-dev

Removes api-ci.yml, web-ci.yml, ios-ci.yml, infra-ci.yml, and
infra-shared-ci.yml. Tests are handled by pr-gate.yml, deployments
by cd-dev.yml (dev) and cd-prod.yml (prod)."
```

---

### Task 5: Validate all workflows and document prerequisites

**Files:**
- Verify: `.github/workflows/pr-gate.yml`
- Verify: `.github/workflows/cd-dev.yml`
- Verify: `.github/workflows/cd-prod.yml`

Final validation pass across all three workflows.

- [ ] **Step 1: Validate all YAML files parse correctly**

```bash
for f in .github/workflows/*.yml; do
  echo "Validating $f..."
  python3 -c "import yaml; yaml.safe_load(open('$f'))"
done
echo "All workflows valid"
```

Expected: "All workflows valid" with no errors.

- [ ] **Step 2: Verify workflow cross-references are consistent**

Check that no remaining workflow file references a deleted file:

```bash
grep -r "api-ci\|web-ci\|ios-ci\|infra-ci\|infra-shared-ci" .github/workflows/
```

Expected: No output (no stale references).

- [ ] **Step 3: Verify job dependency graph in cd-dev.yml**

Manually verify these dependency chains are correct in the file:
- `infra-shared` → needs: `changes`
- `infra-dev` → needs: `changes, infra-shared`
- `api-image` → needs: `changes`
- `api-deploy` → needs: `changes, api-image, infra-dev`
- `web-deploy` → needs: `changes, infra-dev`

Deploy jobs that depend on optional upstream jobs (`api-deploy`, `web-deploy`) must have `if: !failure() && !cancelled() && ...` to tolerate skipped upstream jobs. `infra-dev` must NOT use this pattern — it should block on `infra-shared` failure and uses a simple `if: needs.changes.outputs.infra == 'true'`.

- [ ] **Step 4: Commit final state (if any fixes were needed)**

```bash
git add .github/workflows/
git commit -m "ci: fix workflow validation issues"
```

Only commit if fixes were needed in steps 1-3.

---

### Task 6: Manual prerequisites (not automatable)

These must be done by the repo owner in the GitHub and Azure portals before the new workflows will function.

- [ ] **Step 1: Configure squash merges**

GitHub → repo Settings → General → Pull Requests → check "Allow squash merging", uncheck or de-prioritize other merge types.

- [ ] **Step 2: Create `development` GitHub environment**

GitHub → repo Settings → Environments → New environment → name: `development` → Create. No protection rules needed.

- [ ] **Step 3: Add OIDC federated credential for `development` environment**

Azure Portal → Azure AD → App registrations → [your app] → Certificates & secrets → Federated credentials → Add credential:
- Federated credential scenario: GitHub Actions deploying Azure resources
- Organization: your GitHub org/username
- Repository: town-crier
- Entity type: Environment
- Environment name: `development`
- Name: `github-actions-development` (or similar)

- [ ] **Step 4: Verify the `production` environment already exists**

GitHub → repo Settings → Environments → confirm `production` is listed. This is already used by cd-prod.yml.

- [ ] **Step 5: Optionally retire `AZURE_STATIC_WEB_APPS_API_TOKEN` secret**

GitHub → repo Settings → Secrets → delete `AZURE_STATIC_WEB_APPS_API_TOKEN`. cd-dev now uses dynamic token fetch. Only delete after verifying cd-dev works end-to-end.
