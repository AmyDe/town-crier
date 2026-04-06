# Static Authority List Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the PlanIT-fetched authority list with a static JSON file embedded in the binary, eliminating pagination instability that causes missing authorities (e.g., Kingston).

**Architecture:** A static `authorities.json` file is added as an embedded resource in the infrastructure project. A new `StaticAuthorityProvider` loads it once at construction time. The existing `CachedPlanItAuthorityProvider` and its area-specific models are deleted.

**Tech Stack:** .NET 10, System.Text.Json source generators, TUnit

---

### Task 1: Create the static authorities.json data file

**Files:**
- Create: `api/src/town-crier.infrastructure/Authorities/authorities.json`

- [ ] **Step 1: Generate the JSON file from PlanIT's current data**

Run:
```bash
curl -s "https://www.planit.org.uk/api/areas/json?pg_sz=1000&page=1&select=area_id,area_name,area_type" | python3 -c "
import json, sys
data = json.load(sys.stdin)
records = data['records']
entries = [{'id': r['area_id'], 'name': r['area_name'], 'areaType': r['area_type']} for r in records]
entries.sort(key=lambda x: x['name'].lower())
print(json.dumps(entries, indent=2))
" > api/src/town-crier.infrastructure/Authorities/authorities.json
```

Expected: A JSON file with 485 authority objects, alphabetically sorted by name. Verify Kingston is present:
```bash
grep -c "Kingston" api/src/town-crier.infrastructure/Authorities/authorities.json
```
Expected: 2 matches (Kingston, Kingston upon Hull — or similar)

- [ ] **Step 2: Register as embedded resource**

In `api/src/town-crier.infrastructure/town-crier.infrastructure.csproj`, add to the existing `<ItemGroup>` that contains `EmbeddedResource`:

```xml
<EmbeddedResource Include="Authorities\authorities.json" />
```

The existing item group already has `Geocoding\authority-mapping.json`, so add the new line alongside it.

- [ ] **Step 3: Commit**

```bash
git add api/src/town-crier.infrastructure/Authorities/authorities.json api/src/town-crier.infrastructure/town-crier.infrastructure.csproj
git commit -m "feat: add static authorities.json data file with 485 UK local authorities"
```

---

### Task 2: Create StaticAuthorityProvider with tests (TDD)

**Files:**
- Create: `api/src/town-crier.infrastructure/Authorities/StaticAuthorityProvider.cs`
- Create: `api/src/town-crier.infrastructure/Authorities/AuthorityRecord.cs`
- Create: `api/src/town-crier.infrastructure/Authorities/AuthorityJsonSerializerContext.cs`
- Create: `api/tests/town-crier.infrastructure.tests/Authorities/StaticAuthorityProviderTests.cs`

- [ ] **Step 1: Write the failing test — loads all authorities**

Create `api/tests/town-crier.infrastructure.tests/Authorities/StaticAuthorityProviderTests.cs`:

```csharp
namespace TownCrier.Infrastructure.Tests.Authorities;

public sealed class StaticAuthorityProviderTests
{
    [Test]
    public async Task Should_LoadAllAuthorities_From_EmbeddedJson()
    {
        // Arrange
        var provider = new Infrastructure.Authorities.StaticAuthorityProvider();

        // Act
        var authorities = await provider.GetAllAsync(CancellationToken.None);

        // Assert — the embedded JSON has 485 authorities
        await Assert.That(authorities.Count).IsGreaterThan(400);
    }

    [Test]
    public async Task Should_ContainKingston_When_Loaded()
    {
        // Arrange
        var provider = new Infrastructure.Authorities.StaticAuthorityProvider();

        // Act
        var authorities = await provider.GetAllAsync(CancellationToken.None);

        // Assert
        await Assert.That(authorities.Any(a => a.Name == "Kingston")).IsTrue();
    }

    [Test]
    public async Task Should_ReturnAuthority_When_GetByIdWithValidId()
    {
        // Arrange
        var provider = new Infrastructure.Authorities.StaticAuthorityProvider();

        // Act — Kingston has ID 314
        var authority = await provider.GetByIdAsync(314, CancellationToken.None);

        // Assert
        await Assert.That(authority).IsNotNull();
        await Assert.That(authority!.Name).IsEqualTo("Kingston");
    }

    [Test]
    public async Task Should_ReturnNull_When_GetByIdWithInvalidId()
    {
        // Arrange
        var provider = new Infrastructure.Authorities.StaticAuthorityProvider();

        // Act
        var authority = await provider.GetByIdAsync(99999, CancellationToken.None);

        // Assert
        await Assert.That(authority).IsNull();
    }

    [Test]
    public async Task Should_SortAlphabetically_When_Loaded()
    {
        // Arrange
        var provider = new Infrastructure.Authorities.StaticAuthorityProvider();

        // Act
        var authorities = await provider.GetAllAsync(CancellationToken.None);

        // Assert — first authority alphabetically should be Aberdeen or similar
        var names = authorities.Select(a => a.Name).ToList();
        var sorted = names.OrderBy(n => n, StringComparer.OrdinalIgnoreCase).ToList();
        await Assert.That(names).IsEquivalentTo(sorted);
    }

    [Test]
    public async Task Should_SetCouncilUrlAndPlanningUrlToNull()
    {
        // Arrange
        var provider = new Infrastructure.Authorities.StaticAuthorityProvider();

        // Act
        var authority = await provider.GetByIdAsync(314, CancellationToken.None);

        // Assert
        await Assert.That(authority).IsNotNull();
        await Assert.That(authority!.CouncilUrl).IsNull();
        await Assert.That(authority.PlanningUrl).IsNull();
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `dotnet test api/tests/town-crier.infrastructure.tests --filter "StaticAuthorityProvider" --no-restore`
Expected: FAIL — `StaticAuthorityProvider` does not exist yet.

- [ ] **Step 3: Create the AuthorityRecord DTO**

Create `api/src/town-crier.infrastructure/Authorities/AuthorityRecord.cs`:

```csharp
using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Authorities;

internal sealed class AuthorityRecord
{
    [JsonPropertyName("id")]
    public int Id { get; set; }

    [JsonPropertyName("name")]
    public string Name { get; set; } = string.Empty;

    [JsonPropertyName("areaType")]
    public string AreaType { get; set; } = string.Empty;
}
```

- [ ] **Step 4: Create the JSON serializer context (Native AOT)**

Create `api/src/town-crier.infrastructure/Authorities/AuthorityJsonSerializerContext.cs`:

```csharp
using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Authorities;

[JsonSerializable(typeof(List<AuthorityRecord>))]
internal sealed partial class AuthorityJsonSerializerContext : JsonSerializerContext;
```

- [ ] **Step 5: Implement StaticAuthorityProvider**

Create `api/src/town-crier.infrastructure/Authorities/StaticAuthorityProvider.cs`:

```csharp
using System.Reflection;
using System.Text.Json;
using TownCrier.Application.Authorities;
using TownCrier.Domain.Authorities;

namespace TownCrier.Infrastructure.Authorities;

public sealed class StaticAuthorityProvider : IAuthorityProvider
{
    private readonly IReadOnlyList<Authority> authorities;
    private readonly Dictionary<int, Authority> authoritiesById;

    public StaticAuthorityProvider()
    {
        var records = LoadEmbeddedAuthorities();

        this.authorities = records
            .Select(r => new Authority(r.Id, r.Name, r.AreaType, councilUrl: null, planningUrl: null))
            .OrderBy(a => a.Name, StringComparer.OrdinalIgnoreCase)
            .ToList()
            .AsReadOnly();

        this.authoritiesById = this.authorities.ToDictionary(a => a.Id);
    }

    public Task<IReadOnlyList<Authority>> GetAllAsync(CancellationToken ct)
    {
        return Task.FromResult(this.authorities);
    }

    public Task<Authority?> GetByIdAsync(int id, CancellationToken ct)
    {
        this.authoritiesById.TryGetValue(id, out var authority);
        return Task.FromResult(authority);
    }

    private static List<AuthorityRecord> LoadEmbeddedAuthorities()
    {
        var assembly = Assembly.GetExecutingAssembly();
        const string resourceName = "TownCrier.Infrastructure.Authorities.authorities.json";

        using var stream = assembly.GetManifestResourceStream(resourceName)
            ?? throw new InvalidOperationException($"Embedded resource '{resourceName}' not found.");

        return JsonSerializer.Deserialize(stream, AuthorityJsonSerializerContext.Default.ListAuthorityRecord)
            ?? throw new InvalidOperationException("Failed to deserialize authorities.");
    }
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `dotnet test api/tests/town-crier.infrastructure.tests --filter "StaticAuthorityProvider" --no-restore`
Expected: All 6 tests PASS.

- [ ] **Step 7: Commit**

```bash
git add api/src/town-crier.infrastructure/Authorities/ api/tests/town-crier.infrastructure.tests/Authorities/
git commit -m "feat: add StaticAuthorityProvider backed by embedded JSON"
```

---

### Task 3: Wire up StaticAuthorityProvider and remove CachedPlanItAuthorityProvider

**Files:**
- Modify: `api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs:91-97`
- Delete: `api/src/town-crier.infrastructure/PlanIt/CachedPlanItAuthorityProvider.cs`
- Delete: `api/src/town-crier.infrastructure/PlanIt/PlanItAreaRecord.cs`
- Delete: `api/src/town-crier.infrastructure/PlanIt/PlanItAreasResponse.cs`
- Modify: `api/src/town-crier.infrastructure/PlanIt/PlanItJsonSerializerContext.cs`
- Delete: `api/tests/town-crier.infrastructure.tests/PlanIt/CachedPlanItAuthorityProviderTests.cs`

- [ ] **Step 1: Replace the DI registration**

In `api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs`, replace lines 91-97:

```csharp
        services.AddSingleton<IAuthorityProvider>(sp =>
        {
            var factory = sp.GetRequiredService<IHttpClientFactory>();
            var httpClient = factory.CreateClient("PlanItAreas");
            httpClient.BaseAddress = new Uri(planItBaseUrl);
            return new CachedPlanItAuthorityProvider(httpClient, sp.GetRequiredService<TimeProvider>());
        });
```

With:

```csharp
        services.AddSingleton<IAuthorityProvider>(new StaticAuthorityProvider());
```

Add this using statement at the top of the file:

```csharp
using TownCrier.Infrastructure.Authorities;
```

- [ ] **Step 2: Remove area types from PlanItJsonSerializerContext**

In `api/src/town-crier.infrastructure/PlanIt/PlanItJsonSerializerContext.cs`, remove the two lines:

```csharp
[JsonSerializable(typeof(PlanItAreasResponse))]
[JsonSerializable(typeof(List<PlanItAreaRecord>))]
```

The file should end up with only:

```csharp
using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.PlanIt;

[JsonSerializable(typeof(PlanItResponse))]
[JsonSerializable(typeof(List<PlanItApplicationRecord>))]
internal sealed partial class PlanItJsonSerializerContext : JsonSerializerContext;
```

- [ ] **Step 3: Delete removed files**

```bash
rm -f api/src/town-crier.infrastructure/PlanIt/CachedPlanItAuthorityProvider.cs
rm -f api/src/town-crier.infrastructure/PlanIt/PlanItAreaRecord.cs
rm -f api/src/town-crier.infrastructure/PlanIt/PlanItAreasResponse.cs
rm -f api/tests/town-crier.infrastructure.tests/PlanIt/CachedPlanItAuthorityProviderTests.cs
```

- [ ] **Step 4: Build to verify no compilation errors**

Run: `dotnet build api/`
Expected: Build succeeded with 0 errors. There may be warnings but no errors.

- [ ] **Step 5: Run all tests to verify nothing broke**

Run: `dotnet test api/`
Expected: All tests pass. The old `CachedPlanItAuthorityProviderTests` are gone; the new `StaticAuthorityProviderTests` pass; the existing `GetAuthoritiesQueryHandlerTests` still pass (they use `FakeAuthorityProvider` and are unaffected).

- [ ] **Step 6: Commit**

```bash
git add -A api/
git commit -m "refactor: replace PlanIT authority fetching with static embedded JSON

Fixes missing authorities (e.g., Kingston) caused by PlanIT's
unstable pagination returning duplicates and dropping records."
```
