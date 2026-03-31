# Integration Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add API-level integration tests that prove the deployed stack works end-to-end (Auth0 + API + Cosmos DB), run them against a staging revision in the PR gate, and promote on success.

**Architecture:** A standalone TUnit test project that uses HttpClient with real Auth0 tokens to call the deployed API. The PR gate deploys a staging Container App revision (0% traffic), runs tests against it via a revision label URL, and promotes on success.

**Tech Stack:** .NET 10 (TUnit), Azure Container Apps multi-revision mode, Auth0 Resource Owner Password Grant, GitHub Actions

**Spec:** `docs/superpowers/specs/2026-03-30-integration-tests-design.md`

---

## Prerequisites (Manual, One-Time)

Before implementing, complete these Auth0 and GitHub setup steps:

1. **Auth0: Create test user** — In the Auth0 dashboard, create a user in the dev tenant database connection with email `integration-test@towncrierapp.uk` and a strong password.

2. **Auth0: Enable Password grant** — In the Auth0 dashboard → Applications → (your SPA app) → Settings → Advanced → Grant Types → enable "Password". Also ensure the database connection is enabled for this application under the "Connections" tab.

3. **GitHub: Add secrets** — In the repository settings → Secrets and variables → Actions:
   - Secret: `INTEGRATION_TEST_USERNAME` = `integration-test@towncrierapp.uk`
   - Secret: `INTEGRATION_TEST_PASSWORD` = (the password you set)

4. **GitHub: Add variables** — In the repository settings → Secrets and variables → Actions → Variables:
   - Variable: `AUTH0_CLIENT_SECRET` = (the client secret from Auth0 app settings, if the app has one — SPAs may not)

---

## File Structure

### New files

| File | Responsibility |
|------|----------------|
| `api/tests/town-crier.integration-tests/town-crier.integration-tests.csproj` | Project file — TUnit, HttpClient |
| `api/tests/town-crier.integration-tests/IntegrationTestConfig.cs` | Reads configuration from environment variables |
| `api/tests/town-crier.integration-tests/Auth0TokenProvider.cs` | Acquires Auth0 token via Resource Owner Password Grant |
| `api/tests/town-crier.integration-tests/ApiClientFixture.cs` | Provides an authenticated HttpClient, shared across tests |
| `api/tests/town-crier.integration-tests/HealthTests.cs` | Health endpoint smoke test |
| `api/tests/town-crier.integration-tests/UserProfileTests.cs` | Profile create + read smoke test |
| `api/tests/town-crier.integration-tests/WatchZoneTests.cs` | Watch zone create + list smoke test |

### Modified files

| File | Change |
|------|--------|
| `api/town-crier.sln` | Add integration test project |
| `.github/workflows/pr-gate.yml` | Add staging deploy + integration test + promote jobs; exclude integration tests from unit test run; add permissions |
| `infra/EnvironmentStack.cs` | Set `ActiveRevisionsMode.Multiple`, add traffic to `IgnoreChanges` |

---

## Task 1: Create Integration Test Project

**Files:**
- Create: `api/tests/town-crier.integration-tests/town-crier.integration-tests.csproj`
- Modify: `api/town-crier.sln`

- [ ] **Step 1: Create the project file**

```xml
<Project Sdk="Microsoft.NET.Sdk">

  <PropertyGroup>
    <TargetFramework>net10.0</TargetFramework>
    <RootNamespace>TownCrier.IntegrationTests</RootNamespace>
    <ImplicitUsings>enable</ImplicitUsings>
    <Nullable>enable</Nullable>
    <OutputType>Exe</OutputType>
    <NoWarn>$(NoWarn);CA1812</NoWarn>
  </PropertyGroup>

  <ItemGroup>
    <PackageReference Include="TUnit" Version="0.12.*" />
  </ItemGroup>

</Project>
```

No project references — this project is standalone and talks to the API over HTTP.

- [ ] **Step 2: Add to the solution**

```bash
cd /Users/christy/Dev/town-crier/api
dotnet sln add tests/town-crier.integration-tests/town-crier.integration-tests.csproj --solution-folder tests
```

- [ ] **Step 3: Verify it builds**

```bash
cd /Users/christy/Dev/town-crier/api
dotnet build tests/town-crier.integration-tests
```

Expected: Build succeeded.

- [ ] **Step 4: Commit**

```bash
git add api/tests/town-crier.integration-tests/town-crier.integration-tests.csproj api/town-crier.sln
git commit -m "feat: add integration test project skeleton"
```

---

## Task 2: Configuration and Auth0 Token Provider

**Files:**
- Create: `api/tests/town-crier.integration-tests/IntegrationTestConfig.cs`
- Create: `api/tests/town-crier.integration-tests/Auth0TokenProvider.cs`

- [ ] **Step 1: Create IntegrationTestConfig**

```csharp
namespace TownCrier.IntegrationTests;

internal static class IntegrationTestConfig
{
    public static string ApiBaseUrl =>
        GetRequired("INTEGRATION_TEST_API_BASE_URL");

    public static string Auth0Domain =>
        GetRequired("INTEGRATION_TEST_AUTH0_DOMAIN");

    public static string Auth0ClientId =>
        GetRequired("INTEGRATION_TEST_AUTH0_CLIENT_ID");

    public static string? Auth0ClientSecret =>
        Environment.GetEnvironmentVariable("INTEGRATION_TEST_AUTH0_CLIENT_SECRET");

    public static string Auth0Audience =>
        GetRequired("INTEGRATION_TEST_AUTH0_AUDIENCE");

    public static string Username =>
        GetRequired("INTEGRATION_TEST_USERNAME");

    public static string Password =>
        GetRequired("INTEGRATION_TEST_PASSWORD");

    private static string GetRequired(string name) =>
        Environment.GetEnvironmentVariable(name)
            ?? throw new InvalidOperationException(
                $"Required environment variable '{name}' is not set.");
}
```

- [ ] **Step 2: Create Auth0TokenProvider**

```csharp
using System.Net.Http.Json;
using System.Text.Json.Serialization;

namespace TownCrier.IntegrationTests;

internal static class Auth0TokenProvider
{
    private static readonly Lazy<Task<string>> TokenTask = new(AcquireTokenAsync);

    public static Task<string> GetTokenAsync() => TokenTask.Value;

    private static async Task<string> AcquireTokenAsync()
    {
        using var client = new HttpClient();

        var tokenUrl = $"https://{IntegrationTestConfig.Auth0Domain}/oauth/token";

        var payload = new Dictionary<string, string>
        {
            ["grant_type"] = "password",
            ["client_id"] = IntegrationTestConfig.Auth0ClientId,
            ["username"] = IntegrationTestConfig.Username,
            ["password"] = IntegrationTestConfig.Password,
            ["audience"] = IntegrationTestConfig.Auth0Audience,
            ["scope"] = "openid",
        };

        if (IntegrationTestConfig.Auth0ClientSecret is { } secret)
        {
            payload["client_secret"] = secret;
        }

        using var response = await client.PostAsJsonAsync(tokenUrl, payload).ConfigureAwait(false);

        if (!response.IsSuccessStatusCode)
        {
            var errorBody = await response.Content.ReadAsStringAsync().ConfigureAwait(false);
            throw new InvalidOperationException(
                $"Auth0 token request failed ({response.StatusCode}): {errorBody}");
        }

        var tokenResponse = await response.Content
            .ReadFromJsonAsync<TokenResponse>().ConfigureAwait(false)
            ?? throw new InvalidOperationException("Auth0 returned an empty token response.");

        return tokenResponse.AccessToken;
    }

    private sealed record TokenResponse(
        [property: JsonPropertyName("access_token")] string AccessToken);
}
```

- [ ] **Step 3: Verify it builds**

```bash
cd /Users/christy/Dev/town-crier/api
dotnet build tests/town-crier.integration-tests
```

Expected: Build succeeded.

- [ ] **Step 4: Commit**

```bash
git add api/tests/town-crier.integration-tests/IntegrationTestConfig.cs api/tests/town-crier.integration-tests/Auth0TokenProvider.cs
git commit -m "feat: add integration test config and Auth0 token provider"
```

---

## Task 3: API Client Fixture

**Files:**
- Create: `api/tests/town-crier.integration-tests/ApiClientFixture.cs`

- [ ] **Step 1: Create ApiClientFixture**

This provides a shared, authenticated HttpClient for all integration tests. TUnit's `[ClassDataSource]` ensures it's created once per test run.

```csharp
using System.Net.Http.Headers;

namespace TownCrier.IntegrationTests;

internal sealed class ApiClientFixture : IAsyncInitializer, IAsyncDisposable
{
    private HttpClient? client;

    public HttpClient Client => this.client
        ?? throw new InvalidOperationException("Fixture not initialized.");

    public async Task InitializeAsync()
    {
        var token = await Auth0TokenProvider.GetTokenAsync().ConfigureAwait(false);

        this.client = new HttpClient
        {
            BaseAddress = new Uri(IntegrationTestConfig.ApiBaseUrl),
        };
        this.client.DefaultRequestHeaders.Authorization =
            new AuthenticationHeaderValue("Bearer", token);
    }

    public ValueTask DisposeAsync()
    {
        this.client?.Dispose();
        return ValueTask.CompletedTask;
    }
}
```

- [ ] **Step 2: Verify it builds**

```bash
cd /Users/christy/Dev/town-crier/api
dotnet build tests/town-crier.integration-tests
```

Expected: Build succeeded.

- [ ] **Step 3: Commit**

```bash
git add api/tests/town-crier.integration-tests/ApiClientFixture.cs
git commit -m "feat: add API client fixture for integration tests"
```

---

## Task 4: Health Check Test

**Files:**
- Create: `api/tests/town-crier.integration-tests/HealthTests.cs`

- [ ] **Step 1: Write the health check test**

```csharp
using System.Net;

namespace TownCrier.IntegrationTests;

[NotInParallel]
public sealed class HealthTests
{
    [Test]
    public async Task Should_ReturnHealthy_When_HealthEndpointCalled()
    {
        // Arrange — no auth needed for health
        using var client = new HttpClient
        {
            BaseAddress = new Uri(IntegrationTestConfig.ApiBaseUrl),
        };

        // Act
        using var response = await client.GetAsync(new Uri("/v1/health", UriKind.Relative));

        // Assert
        var body = await response.Content.ReadAsStringAsync();
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);
        await Assert.That(body).Contains("Healthy");
    }
}
```

- [ ] **Step 2: Verify it builds**

```bash
cd /Users/christy/Dev/town-crier/api
dotnet build tests/town-crier.integration-tests
```

Expected: Build succeeded. (Tests won't pass locally without env vars pointing to a running API — that's expected.)

- [ ] **Step 3: Commit**

```bash
git add api/tests/town-crier.integration-tests/HealthTests.cs
git commit -m "feat: add health check integration test"
```

---

## Task 5: User Profile Test

**Files:**
- Create: `api/tests/town-crier.integration-tests/UserProfileTests.cs`

- [ ] **Step 1: Write the profile creation and retrieval test**

```csharp
using System.Net;
using System.Text.Json;

namespace TownCrier.IntegrationTests;

[NotInParallel]
[ClassDataSource<ApiClientFixture>(Shared = SharedType.Globally)]
public sealed class UserProfileTests(ApiClientFixture fixture)
{
    [Test]
    public async Task Should_CreateAndRetrieveProfile_When_Authenticated()
    {
        var client = fixture.Client;

        // Act — create profile (idempotent, returns 200 on subsequent calls)
        using var createResponse = await client.PostAsync(
            new Uri("/v1/me", UriKind.Relative), content: null);

        // Assert — creation succeeds
        var createBody = await createResponse.Content.ReadAsStringAsync();
        await Assert.That(createResponse.StatusCode).IsEqualTo(HttpStatusCode.OK);

        // Act — retrieve profile
        using var getResponse = await client.GetAsync(
            new Uri("/v1/me", UriKind.Relative));

        // Assert — profile exists and contains a userId
        var getBody = await getResponse.Content.ReadAsStringAsync();
        await Assert.That(getResponse.StatusCode).IsEqualTo(HttpStatusCode.OK);

        using var doc = JsonDocument.Parse(getBody);
        var userId = doc.RootElement.GetProperty("userId").GetString();
        await Assert.That(userId).IsNotNull();
        await Assert.That(userId!).IsNotEmpty();
    }
}
```

- [ ] **Step 2: Verify it builds**

```bash
cd /Users/christy/Dev/town-crier/api
dotnet build tests/town-crier.integration-tests
```

Expected: Build succeeded.

- [ ] **Step 3: Commit**

```bash
git add api/tests/town-crier.integration-tests/UserProfileTests.cs
git commit -m "feat: add user profile integration test"
```

---

## Task 6: Watch Zone Test

**Files:**
- Create: `api/tests/town-crier.integration-tests/WatchZoneTests.cs`

- [ ] **Step 1: Write the watch zone creation and retrieval test**

```csharp
using System.Net;
using System.Net.Http.Json;
using System.Text.Json;

namespace TownCrier.IntegrationTests;

[NotInParallel]
[ClassDataSource<ApiClientFixture>(Shared = SharedType.Globally)]
public sealed class WatchZoneTests(ApiClientFixture fixture)
{
    private static readonly JsonSerializerOptions CamelCase = new()
    {
        PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
    };

    [Test]
    public async Task Should_CreateAndListWatchZone_When_Authenticated()
    {
        var client = fixture.Client;
        var zoneId = Guid.NewGuid().ToString();

        // Ensure profile exists (idempotent) — watch zones require a profile
        await client.PostAsync(new Uri("/v1/me", UriKind.Relative), content: null);

        // Arrange — request body (userId is overridden by the API from the JWT)
        var requestBody = new
        {
            userId = "ignored",
            zoneId,
            name = "Integration Test Zone",
            latitude = 51.5074,
            longitude = -0.1278,
            radiusMetres = 1000.0,
        };

        // Act — create watch zone
        using var createResponse = await client.PostAsJsonAsync(
            new Uri("/v1/me/watch-zones", UriKind.Relative), requestBody, CamelCase);

        // Assert — creation succeeds
        var createBody = await createResponse.Content.ReadAsStringAsync();
        await Assert.That(createResponse.StatusCode).IsEqualTo(HttpStatusCode.Created);

        // Act — list watch zones
        using var listResponse = await client.GetAsync(
            new Uri("/v1/me/watch-zones", UriKind.Relative));

        // Assert — list contains the zone we created
        var listBody = await listResponse.Content.ReadAsStringAsync();
        await Assert.That(listResponse.StatusCode).IsEqualTo(HttpStatusCode.OK);

        using var doc = JsonDocument.Parse(listBody);
        var zones = doc.RootElement.GetProperty("zones");
        await Assert.That(zones.GetArrayLength()).IsGreaterThanOrEqualTo(1);

        var createdZone = zones.EnumerateArray()
            .FirstOrDefault(z => z.GetProperty("id").GetString() == zoneId);
        await Assert.That(createdZone.ValueKind).IsNotEqualTo(JsonValueKind.Undefined);
        await Assert.That(createdZone.GetProperty("name").GetString())
            .IsEqualTo("Integration Test Zone");
    }
}
```

- [ ] **Step 2: Verify it builds**

```bash
cd /Users/christy/Dev/town-crier/api
dotnet build tests/town-crier.integration-tests
```

Expected: Build succeeded.

- [ ] **Step 3: Commit**

```bash
git add api/tests/town-crier.integration-tests/WatchZoneTests.cs
git commit -m "feat: add watch zone integration test"
```

---

## Task 7: Exclude Integration Tests from Unit Test Run

**Files:**
- Modify: `.github/workflows/pr-gate.yml` (lines 80)

The `api-build-test` job currently runs `dotnet test` at the solution level, which would include the integration tests. They'd fail because the env vars aren't set. Exclude them with a filter.

- [ ] **Step 1: Update the dotnet test command in the api-build-test job**

Change the test step in the `api-build-test` job from:

```yaml
      - run: dotnet test --no-build --configuration Release
        working-directory: api
```

to:

```yaml
      - run: dotnet test --no-build --configuration Release --filter "FullyQualifiedName!~TownCrier.IntegrationTests"
        working-directory: api
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/pr-gate.yml
git commit -m "fix: exclude integration tests from unit test run"
```

---

## Task 8: Pulumi — Multi-Revision Mode

**Files:**
- Modify: `infra/EnvironmentStack.cs` (lines 201-287)

- [ ] **Step 1: Add ActiveRevisionsMode.Multiple to the Container App configuration**

In `EnvironmentStack.cs`, add the `ActiveRevisionsMode` property to the `ConfigurationArgs` block:

```csharp
            Configuration = new ConfigurationArgs
            {
                ActiveRevisionsMode = ActiveRevisionsMode.Multiple,
                Ingress = new IngressArgs
```

(Insert `ActiveRevisionsMode = ActiveRevisionsMode.Multiple,` as the first property inside `ConfigurationArgs`.)

- [ ] **Step 2: Add traffic to IgnoreChanges**

The CI pipeline manages traffic weights via `az containerapp ingress traffic set`. Pulumi must not reset them. Update the `IgnoreChanges` list:

Change:

```csharp
        }, new CustomResourceOptions
        {
            IgnoreChanges = { "template.containers[0].image" },
        });
```

to:

```csharp
        }, new CustomResourceOptions
        {
            IgnoreChanges =
            {
                "template.containers[0].image",
                "configuration.ingress.traffic",
            },
        });
```

- [ ] **Step 3: Verify Pulumi builds**

```bash
cd /Users/christy/Dev/town-crier/infra
dotnet build
```

Expected: Build succeeded.

- [ ] **Step 4: Commit**

```bash
git add infra/EnvironmentStack.cs
git commit -m "feat: enable multi-revision mode for Container App"
```

---

## Task 9: PR Gate — Staging Deploy, Integration Tests, Promotion

**Files:**
- Modify: `.github/workflows/pr-gate.yml`

This is the largest task. It adds three new jobs to the PR gate workflow: build+deploy a staging revision, run integration tests, and promote on success.

- [ ] **Step 1: Update workflow-level permissions**

The deployment jobs need `id-token: write` for Azure OIDC login. Change:

```yaml
permissions:
  contents: read
```

to:

```yaml
permissions:
  contents: read
  id-token: write
```

- [ ] **Step 2: Add the api-staging job**

Add this job after the `api-build-test` job. It builds the Docker image, pushes to ACR, deploys a staging revision with 0% traffic, and labels it for testing.

```yaml
  # ── API: staging deploy ─────────────────────────────
  api-staging:
    name: "API: Deploy staging revision"
    needs: [changes, api-build-test]
    if: needs.changes.outputs.api == 'true'
    runs-on: ubuntu-latest
    timeout-minutes: 15
    environment: development
    outputs:
      staging-url: ${{ steps.staging-url.outputs.url }}
      new-revision: ${{ steps.deploy.outputs.revision }}
    steps:
      - uses: actions/checkout@v4

      - uses: ./.github/actions/azure-login
        with:
          client-id: ${{ secrets.AZURE_CLIENT_ID }}
          tenant-id: ${{ secrets.AZURE_TENANT_ID }}
          subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}

      - name: Login to ACR
        run: az acr login --name ${{ secrets.ACR_LOGIN_SERVER }}

      - name: Build and push image
        run: |
          IMAGE="${{ secrets.ACR_LOGIN_SERVER }}/town-crier-api:${{ github.event.pull_request.head.sha }}"
          docker build -t "$IMAGE" .
          docker push "$IMAGE"
        working-directory: api

      - name: Get current active revision
        id: current
        run: |
          CURRENT=$(az containerapp ingress traffic show \
            --name ca-town-crier-api-dev \
            --resource-group rg-town-crier-dev \
            --query "[?weight==\`100\`].revisionName | [0]" -o tsv)
          echo "revision=$CURRENT" >> "$GITHUB_OUTPUT"

      - name: Deploy staging revision
        id: deploy
        run: |
          SHORT_SHA=$(echo "${{ github.event.pull_request.head.sha }}" | cut -c1-7)
          az containerapp update \
            --name ca-town-crier-api-dev \
            --resource-group rg-town-crier-dev \
            --image "${{ secrets.ACR_LOGIN_SERVER }}/town-crier-api:${{ github.event.pull_request.head.sha }}" \
            --revision-suffix "pr${SHORT_SHA}"
          echo "revision=ca-town-crier-api-dev--pr${SHORT_SHA}" >> "$GITHUB_OUTPUT"

      - name: Pin traffic to current revision
        run: |
          if [ -n "${{ steps.current.outputs.revision }}" ]; then
            az containerapp ingress traffic set \
              --name ca-town-crier-api-dev \
              --resource-group rg-town-crier-dev \
              --revision-weight "${{ steps.current.outputs.revision }}=100"
          fi

      - name: Label staging revision
        run: |
          az containerapp revision label add \
            --name ca-town-crier-api-dev \
            --resource-group rg-town-crier-dev \
            --revision "${{ steps.deploy.outputs.revision }}" \
            --label staging --yes

      - name: Get staging URL
        id: staging-url
        run: |
          APP_FQDN=$(az containerapp show \
            --name ca-town-crier-api-dev \
            --resource-group rg-town-crier-dev \
            --query "properties.configuration.ingress.fqdn" -o tsv)
          echo "url=https://staging---${APP_FQDN}" >> "$GITHUB_OUTPUT"
```

- [ ] **Step 3: Add the api-integration-test job**

```yaml
  # ── API: integration tests ──────────────────────────
  api-integration-test:
    name: "API: Integration tests"
    needs: [api-staging]
    runs-on: ubuntu-latest
    timeout-minutes: 10
    environment: development
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-dotnet@v4
        with:
          global-json-file: api/global.json

      - name: Run integration tests
        run: dotnet test tests/town-crier.integration-tests --configuration Release
        working-directory: api
        env:
          INTEGRATION_TEST_API_BASE_URL: ${{ needs.api-staging.outputs.staging-url }}
          INTEGRATION_TEST_AUTH0_DOMAIN: ${{ vars.VITE_AUTH0_DOMAIN }}
          INTEGRATION_TEST_AUTH0_CLIENT_ID: ${{ vars.VITE_AUTH0_CLIENT_ID }}
          INTEGRATION_TEST_AUTH0_CLIENT_SECRET: ${{ secrets.AUTH0_CLIENT_SECRET }}
          INTEGRATION_TEST_AUTH0_AUDIENCE: ${{ vars.VITE_AUTH0_AUDIENCE }}
          INTEGRATION_TEST_USERNAME: ${{ secrets.INTEGRATION_TEST_USERNAME }}
          INTEGRATION_TEST_PASSWORD: ${{ secrets.INTEGRATION_TEST_PASSWORD }}

      - name: Deactivate staging revision on failure
        if: failure()
        uses: ./.github/actions/azure-login
        with:
          client-id: ${{ secrets.AZURE_CLIENT_ID }}
          tenant-id: ${{ secrets.AZURE_TENANT_ID }}
          subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}

      - name: Cleanup staging on failure
        if: failure()
        run: |
          az containerapp revision label remove \
            --name ca-town-crier-api-dev \
            --resource-group rg-town-crier-dev \
            --label staging --yes || true
          az containerapp revision deactivate \
            --name ca-town-crier-api-dev \
            --resource-group rg-town-crier-dev \
            --revision "${{ needs.api-staging.outputs.new-revision }}" || true
```

- [ ] **Step 4: Add the api-promote job**

```yaml
  # ── API: promote staging to live ────────────────────
  api-promote:
    name: "API: Promote staging"
    needs: [api-staging, api-integration-test]
    runs-on: ubuntu-latest
    timeout-minutes: 5
    environment: development
    steps:
      - uses: actions/checkout@v4

      - uses: ./.github/actions/azure-login
        with:
          client-id: ${{ secrets.AZURE_CLIENT_ID }}
          tenant-id: ${{ secrets.AZURE_TENANT_ID }}
          subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}

      - name: Shift traffic to new revision
        run: |
          az containerapp ingress traffic set \
            --name ca-town-crier-api-dev \
            --resource-group rg-town-crier-dev \
            --revision-weight "${{ needs.api-staging.outputs.new-revision }}=100"

      - name: Remove staging label
        run: |
          az containerapp revision label remove \
            --name ca-town-crier-api-dev \
            --resource-group rg-town-crier-dev \
            --label staging --yes || true
```

- [ ] **Step 5: Update the gate job dependencies**

Add the new jobs to the gate job's `needs` list and evaluation:

Change:

```yaml
  gate:
    name: PR Gate
    needs: [api-format, api-build-test, ios-lint, ios-build-test, web-lint, web-build-test, infra-preview]
    if: always()
    runs-on: ubuntu-latest
    timeout-minutes: 2
    steps:
      - name: Evaluate results
        run: |
          results=(
            "${{ needs.api-format.result }}"
            "${{ needs.api-build-test.result }}"
            "${{ needs.ios-lint.result }}"
            "${{ needs.ios-build-test.result }}"
            "${{ needs.web-lint.result }}"
            "${{ needs.web-build-test.result }}"
            "${{ needs.infra-preview.result }}"
          )
```

to:

```yaml
  gate:
    name: PR Gate
    needs: [api-format, api-build-test, api-staging, api-integration-test, api-promote, ios-lint, ios-build-test, web-lint, web-build-test, infra-preview]
    if: always()
    runs-on: ubuntu-latest
    timeout-minutes: 2
    steps:
      - name: Evaluate results
        run: |
          results=(
            "${{ needs.api-format.result }}"
            "${{ needs.api-build-test.result }}"
            "${{ needs.api-staging.result }}"
            "${{ needs.api-integration-test.result }}"
            "${{ needs.api-promote.result }}"
            "${{ needs.ios-lint.result }}"
            "${{ needs.ios-build-test.result }}"
            "${{ needs.web-lint.result }}"
            "${{ needs.web-build-test.result }}"
            "${{ needs.infra-preview.result }}"
          )
```

- [ ] **Step 6: Verify the workflow YAML is valid**

```bash
cd /Users/christy/Dev/town-crier
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/pr-gate.yml'))" && echo "Valid YAML"
```

Expected: `Valid YAML`

- [ ] **Step 7: Commit**

```bash
git add .github/workflows/pr-gate.yml
git commit -m "feat: add staging deploy and integration tests to PR gate"
```

---

## Follow-Up Considerations

These are not in scope for this plan but should be tracked:

1. **cd-dev.yml redundancy** — Once the PR gate deploys and promotes, the cd-dev API deployment is partially redundant. It still handles direct pushes to main and infra changes. Consider simplifying it.

2. **Revision cleanup** — Old deactivated revisions accumulate. Azure has a 100-revision limit per Container App. Consider a periodic cleanup step or scheduled job.

3. **Auth0 Password Grant deprecation** — Auth0 considers the Resource Owner Password Grant as legacy. If Auth0 deprecates it, switch to a client-credentials flow with a Machine-to-Machine application, or automate the Authorization Code flow.

4. **Playwright tests** — When ready to add browser-level tests, the staging revision approach works identically — just point Playwright at the staging URL.
