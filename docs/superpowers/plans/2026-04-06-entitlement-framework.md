# Entitlement Framework Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a declarative entitlement framework that gates API endpoints and background job features by subscription tier, using JWT claims for HTTP requests and a shared `EntitlementMap` for all code paths.

**Architecture:** An `Entitlement` enum and `Quota` enum in the domain layer with a static `EntitlementMap` that resolves tier → entitlements/quotas. An `EntitlementEndpointFilter` in the web layer reads the `subscription_tier` JWT claim and short-circuits with 403 if the required entitlement is missing. Background jobs use the same `EntitlementMap` with `UserProfile.Tier` from Cosmos. An Auth0 Post-Login Action writes the claim from `app_metadata`, synced by the API when tier changes.

**Tech Stack:** .NET 10, ASP.NET Core minimal APIs, TUnit, Auth0 Actions (JS), Auth0 Management API

**Spec:** `docs/specs/entitlement-framework.md`

---

## File Map

### New Files
| File | Purpose |
|------|---------|
| `api/src/town-crier.domain/Entitlements/Entitlement.cs` | Enum of gatable features |
| `api/src/town-crier.domain/Entitlements/Quota.cs` | Enum of numeric-limited features |
| `api/src/town-crier.domain/Entitlements/EntitlementMap.cs` | Static tier→entitlements and tier→quotas mapping |
| `api/src/town-crier.web/Endpoints/RequiresEntitlementAttribute.cs` | Attribute to mark entitled endpoints |
| `api/src/town-crier.web/Endpoints/EntitlementEndpointFilter.cs` | Filter that reads JWT claim and checks entitlement |
| `api/src/town-crier.web/Endpoints/EntitlementErrorResponse.cs` | Structured 403 response body |
| `api/src/town-crier.application/Auth/IAuth0ManagementClient.cs` | Port for Auth0 metadata sync |
| `api/src/town-crier.infrastructure/Auth/Auth0ManagementClient.cs` | Adapter calling Auth0 Management API |
| `api/src/town-crier.infrastructure/Auth/NoOpAuth0ManagementClient.cs` | No-op for dev/test |
| `api/tests/town-crier.domain.tests/Entitlements/EntitlementMapTests.cs` | Tests for tier→entitlement resolution |
| `api/tests/town-crier.web.tests/Entitlements/EntitlementEndpointFilterTests.cs` | Tests for the filter |
| `api/tests/town-crier.application.tests/Admin/GrantSubscriptionCommandHandlerTests.cs` | Tests for Auth0 sync |

### Modified Files
| File | Change |
|------|--------|
| `api/src/town-crier.web/AppJsonSerializerContext.cs` | Register `EntitlementErrorResponse` |
| `api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs` | Register `IAuth0ManagementClient` |
| `api/src/town-crier.web/Endpoints/UserProfileEndpoints.cs` | Add entitlement filter to PATCH `/me` |
| `api/src/town-crier.application/Admin/GrantSubscriptionCommandHandler.cs` | Add Auth0 metadata sync |
| `api/src/town-crier.application/Notifications/DispatchNotificationCommandHandler.cs` | Use `EntitlementMap` |
| `api/src/town-crier.application/Notifications/GenerateWeeklyDigestsCommandHandler.cs` | Use `EntitlementMap` |

---

## Task 1: Entitlement and Quota Enums

**Files:**
- Create: `api/src/town-crier.domain/Entitlements/Entitlement.cs`
- Create: `api/src/town-crier.domain/Entitlements/Quota.cs`

- [ ] **Step 1: Create the Entitlement enum**

```csharp
// api/src/town-crier.domain/Entitlements/Entitlement.cs
namespace TownCrier.Domain.Entitlements;

public enum Entitlement
{
    InstantEmails,
    SearchApplications,
    StatusChangeAlerts,
    DecisionUpdateAlerts,
}
```

- [ ] **Step 2: Create the Quota enum**

```csharp
// api/src/town-crier.domain/Entitlements/Quota.cs
namespace TownCrier.Domain.Entitlements;

public enum Quota
{
    WatchZones,
}
```

- [ ] **Step 3: Verify build**

Run: `dotnet build api/src/town-crier.domain/town-crier.domain.csproj`
Expected: Build succeeded

- [ ] **Step 4: Commit**

```bash
git add api/src/town-crier.domain/Entitlements/
git commit -m "feat: add Entitlement and Quota enums"
```

---

## Task 2: EntitlementMap with Tests

**Files:**
- Create: `api/src/town-crier.domain/Entitlements/EntitlementMap.cs`
- Create: `api/tests/town-crier.domain.tests/Entitlements/EntitlementMapTests.cs`

- [ ] **Step 1: Write the failing tests**

```csharp
// api/tests/town-crier.domain.tests/Entitlements/EntitlementMapTests.cs
using TownCrier.Domain.Entitlements;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Domain.Tests.Entitlements;

public sealed class EntitlementMapTests
{
    [Test]
    public async Task FreeTier_Should_HaveNoEntitlements()
    {
        var entitlements = EntitlementMap.EntitlementsFor(SubscriptionTier.Free);

        await Assert.That(entitlements).HasCount().EqualTo(0);
    }

    [Test]
    public async Task PersonalTier_Should_HaveInstantEmailsAndAlerts()
    {
        var entitlements = EntitlementMap.EntitlementsFor(SubscriptionTier.Personal);

        await Assert.That(entitlements).Contains(Entitlement.InstantEmails);
        await Assert.That(entitlements).Contains(Entitlement.StatusChangeAlerts);
        await Assert.That(entitlements).Contains(Entitlement.DecisionUpdateAlerts);
        await Assert.That(entitlements).DoesNotContain(Entitlement.SearchApplications);
    }

    [Test]
    public async Task ProTier_Should_HaveAllEntitlements()
    {
        var entitlements = EntitlementMap.EntitlementsFor(SubscriptionTier.Pro);

        await Assert.That(entitlements).Contains(Entitlement.InstantEmails);
        await Assert.That(entitlements).Contains(Entitlement.SearchApplications);
        await Assert.That(entitlements).Contains(Entitlement.StatusChangeAlerts);
        await Assert.That(entitlements).Contains(Entitlement.DecisionUpdateAlerts);
    }

    [Test]
    public async Task FreeTier_WatchZoneLimit_Should_Be1()
    {
        var limit = EntitlementMap.LimitFor(SubscriptionTier.Free, Quota.WatchZones);

        await Assert.That(limit).IsEqualTo(1);
    }

    [Test]
    public async Task PersonalTier_WatchZoneLimit_Should_Be3()
    {
        var limit = EntitlementMap.LimitFor(SubscriptionTier.Personal, Quota.WatchZones);

        await Assert.That(limit).IsEqualTo(3);
    }

    [Test]
    public async Task ProTier_WatchZoneLimit_Should_BeUnlimited()
    {
        var limit = EntitlementMap.LimitFor(SubscriptionTier.Pro, Quota.WatchZones);

        await Assert.That(limit).IsEqualTo(int.MaxValue);
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `dotnet test api/tests/town-crier.domain.tests/ --filter "EntitlementMap"`
Expected: FAIL — `EntitlementMap` does not exist

- [ ] **Step 3: Implement EntitlementMap**

```csharp
// api/src/town-crier.domain/Entitlements/EntitlementMap.cs
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Domain.Entitlements;

public static class EntitlementMap
{
    private static readonly IReadOnlySet<Entitlement> EmptySet =
        new HashSet<Entitlement>();

    private static readonly IReadOnlySet<Entitlement> PersonalEntitlements =
        new HashSet<Entitlement>
        {
            Entitlement.InstantEmails,
            Entitlement.StatusChangeAlerts,
            Entitlement.DecisionUpdateAlerts,
        };

    private static readonly IReadOnlySet<Entitlement> ProEntitlements =
        new HashSet<Entitlement>
        {
            Entitlement.InstantEmails,
            Entitlement.SearchApplications,
            Entitlement.StatusChangeAlerts,
            Entitlement.DecisionUpdateAlerts,
        };

    public static IReadOnlySet<Entitlement> EntitlementsFor(SubscriptionTier tier) => tier switch
    {
        SubscriptionTier.Personal => PersonalEntitlements,
        SubscriptionTier.Pro => ProEntitlements,
        _ => EmptySet,
    };

    public static int LimitFor(SubscriptionTier tier, Quota quota) => (tier, quota) switch
    {
        (SubscriptionTier.Free, Quota.WatchZones) => 1,
        (SubscriptionTier.Personal, Quota.WatchZones) => 3,
        (SubscriptionTier.Pro, Quota.WatchZones) => int.MaxValue,
        _ => 1,
    };
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `dotnet test api/tests/town-crier.domain.tests/ --filter "EntitlementMap"`
Expected: All 6 tests PASS

- [ ] **Step 5: Commit**

```bash
git add api/src/town-crier.domain/Entitlements/EntitlementMap.cs api/tests/town-crier.domain.tests/Entitlements/EntitlementMapTests.cs
git commit -m "feat: add EntitlementMap with tier-to-entitlement and quota resolution"
```

---

## Task 3: EntitlementEndpointFilter with Tests

**Files:**
- Create: `api/src/town-crier.web/Endpoints/RequiresEntitlementAttribute.cs`
- Create: `api/src/town-crier.web/Endpoints/EntitlementErrorResponse.cs`
- Create: `api/src/town-crier.web/Endpoints/EntitlementEndpointFilter.cs`
- Modify: `api/src/town-crier.web/AppJsonSerializerContext.cs`
- Create: `api/tests/town-crier.web.tests/Entitlements/EntitlementEndpointFilterTests.cs`

- [ ] **Step 1: Write the failing tests**

These tests create a minimal ASP.NET test server with a dummy endpoint protected by the filter, then exercise it with different JWT claims.

```csharp
// api/tests/town-crier.web.tests/Entitlements/EntitlementEndpointFilterTests.cs
using System.Net;
using System.Net.Http.Headers;
using System.Security.Claims;
using TownCrier.Domain.Entitlements;
using TownCrier.Web.Tests.Auth;

namespace TownCrier.Web.Tests.Entitlements;

public sealed class EntitlementEndpointFilterTests
{
    [Test]
    public async Task Should_Return403_When_FreeTierAccessesProFeature()
    {
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        var token = TestJwtToken.Generate(claims:
        [
            new Claim("subscription_tier", "Free"),
        ]);
        client.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", token);

        var response = await client.GetAsync("/v1/search?q=test&authorityId=42&page=1");

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.Forbidden);
    }

    [Test]
    public async Task Should_AllowAccess_When_ProTierAccessesProFeature()
    {
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        var token = TestJwtToken.Generate(claims:
        [
            new Claim("subscription_tier", "Pro"),
        ]);
        client.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", token);

        var response = await client.GetAsync("/v1/search?q=test&authorityId=42&page=1");

        // May be 404 (no user profile) but should NOT be 403
        await Assert.That(response.StatusCode).IsNotEqualTo(HttpStatusCode.Forbidden);
    }

    [Test]
    public async Task Should_DefaultToFree_When_TierClaimMissing()
    {
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        // Token with no subscription_tier claim
        var token = TestJwtToken.Generate();
        client.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", token);

        var response = await client.GetAsync("/v1/search?q=test&authorityId=42&page=1");

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.Forbidden);
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `dotnet test api/tests/town-crier.web.tests/ --filter "EntitlementEndpointFilter"`
Expected: FAIL — types do not exist yet

- [ ] **Step 3: Create the RequiresEntitlementAttribute**

```csharp
// api/src/town-crier.web/Endpoints/RequiresEntitlementAttribute.cs
using TownCrier.Domain.Entitlements;

namespace TownCrier.Web.Endpoints;

[AttributeUsage(AttributeTargets.Method | AttributeTargets.Class)]
internal sealed class RequiresEntitlementAttribute(Entitlement entitlement) : Attribute
{
    public Entitlement Entitlement { get; } = entitlement;
}
```

- [ ] **Step 4: Create the EntitlementErrorResponse**

```csharp
// api/src/town-crier.web/Endpoints/EntitlementErrorResponse.cs
using System.Text.Json.Serialization;

namespace TownCrier.Web.Endpoints;

internal sealed record EntitlementErrorResponse(
    [property: JsonPropertyName("error")] string Error,
    [property: JsonPropertyName("required")] string Required,
    [property: JsonPropertyName("message")] string Message);
```

- [ ] **Step 5: Register EntitlementErrorResponse in the serializer context**

In `api/src/town-crier.web/AppJsonSerializerContext.cs`, add after the existing `ApiErrorResponse` line (line 19):

```csharp
[JsonSerializable(typeof(EntitlementErrorResponse))]
```

Add the using at the top:

```csharp
using TownCrier.Web.Endpoints;
```

- [ ] **Step 6: Create the EntitlementEndpointFilter**

```csharp
// api/src/town-crier.web/Endpoints/EntitlementEndpointFilter.cs
using TownCrier.Domain.Entitlements;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Web.Endpoints;

internal sealed class EntitlementEndpointFilter : IEndpointFilter
{
    public async ValueTask<object?> InvokeAsync(
        EndpointFilterInvocationContext context,
        EndpointFilterDelegate next)
    {
        var endpoint = context.HttpContext.GetEndpoint();
        var attribute = endpoint?.Metadata.GetMetadata<RequiresEntitlementAttribute>();
        if (attribute is null)
        {
            return await next(context).ConfigureAwait(false);
        }

        var tierClaim = context.HttpContext.User.FindFirst("subscription_tier")?.Value;
        var tier = Enum.TryParse<SubscriptionTier>(tierClaim, ignoreCase: true, out var parsed)
            ? parsed
            : SubscriptionTier.Free;

        var entitlements = EntitlementMap.EntitlementsFor(tier);
        if (!entitlements.Contains(attribute.Entitlement))
        {
            return Results.Json(
                new EntitlementErrorResponse(
                    "insufficient_entitlement",
                    attribute.Entitlement.ToString(),
                    "This feature requires a paid subscription."),
                AppJsonSerializerContext.Default.EntitlementErrorResponse,
                statusCode: 403);
        }

        return await next(context).ConfigureAwait(false);
    }
}
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `dotnet test api/tests/town-crier.web.tests/ --filter "EntitlementEndpointFilter"`
Expected: FAIL — the filter is not wired to any endpoints yet. The tests use the `/v1/search` endpoint which still uses the old `ProTierRequiredException` pattern. We need Task 4 first to wire it up. For now, verify the project builds:

Run: `dotnet build api/src/town-crier.web/town-crier.web.csproj`
Expected: Build succeeded

- [ ] **Step 8: Commit**

```bash
git add api/src/town-crier.web/Endpoints/RequiresEntitlementAttribute.cs api/src/town-crier.web/Endpoints/EntitlementErrorResponse.cs api/src/town-crier.web/Endpoints/EntitlementEndpointFilter.cs api/src/town-crier.web/AppJsonSerializerContext.cs
git commit -m "feat: add EntitlementEndpointFilter with RequiresEntitlement attribute"
```

---

## Task 4: Wire Entitlement Filter to Search Endpoint

**Files:**
- Modify: `api/src/town-crier.web/Endpoints/SearchEndpoints.cs`
- Modify: `api/src/town-crier.application/Search/SearchPlanningApplicationsQueryHandler.cs`

This migrates the first endpoint from the ad-hoc `ProTierRequiredException` pattern to the declarative filter. The filter gates before the handler runs, so the handler's tier check becomes redundant and is removed.

- [ ] **Step 1: Add entitlement filter and attribute to the search endpoint**

Replace the contents of `api/src/town-crier.web/Endpoints/SearchEndpoints.cs`:

```csharp
using System.Security.Claims;
using TownCrier.Application.Search;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.Entitlements;

namespace TownCrier.Web.Endpoints;

internal static class SearchEndpoints
{
    public static void MapSearchEndpoints(this RouteGroupBuilder group)
    {
        group.MapGet("/search", async (
            ClaimsPrincipal user,
            string q,
            int authorityId,
            int page,
            SearchPlanningApplicationsQueryHandler handler,
            CancellationToken ct) =>
        {
            var userId = user.FindFirstValue("sub")!;
            var query = new SearchPlanningApplicationsQuery(userId, q, authorityId, page);

            try
            {
                var result = await handler.HandleAsync(query, ct).ConfigureAwait(false);
                return Results.Ok(result);
            }
            catch (UserProfileNotFoundException)
            {
                return Results.NotFound();
            }
        })
        .AddEndpointFilter<EntitlementEndpointFilter>()
        .WithMetadata(new RequiresEntitlementAttribute(Entitlement.SearchApplications));
    }
}
```

Key changes: removed `ProTierRequiredException` catch block, added `.AddEndpointFilter<EntitlementEndpointFilter>()` and `.WithMetadata(new RequiresEntitlementAttribute(Entitlement.SearchApplications))`.

- [ ] **Step 2: Remove the tier check from the search handler**

In `api/src/town-crier.application/Search/SearchPlanningApplicationsQueryHandler.cs`, remove the tier check block (approximately lines 34-37). The handler should no longer throw `ProTierRequiredException` — the filter handles this before the handler runs.

Find and remove:

```csharp
        if (profile.Tier != SubscriptionTier.Pro)
        {
            throw new ProTierRequiredException();
        }
```

Also remove the `using TownCrier.Domain.UserProfiles;` import if it becomes unused.

- [ ] **Step 3: Run the entitlement filter tests**

Run: `dotnet test api/tests/town-crier.web.tests/ --filter "EntitlementEndpointFilter"`
Expected: All 3 tests PASS

- [ ] **Step 4: Run the existing search handler tests**

Run: `dotnet test api/tests/town-crier.application.tests/ --filter "SearchPlanningApplications"`
Expected: The test `Should_ThrowProTierRequired_When_UserIsFreeTier` will now FAIL because the handler no longer throws. Update it:

In `api/tests/town-crier.application.tests/Search/SearchPlanningApplicationsQueryHandlerTests.cs`, replace the first test (lines 12-32):

```csharp
    [Test]
    public async Task Should_ReturnResults_When_UserIsFreeTier()
    {
        // Arrange — tier check is now in the endpoint filter, not the handler
        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Free)
            .Build();
        var userProfileRepository = new FakeUserProfileRepository();
        await userProfileRepository.SaveAsync(profile, CancellationToken.None);

        var planItClient = new FakePlanItClient();
        planItClient.SearchTotal = 0;
        var appRepo = new FakePlanningApplicationRepository();
        var handler = new SearchPlanningApplicationsQueryHandler(userProfileRepository, planItClient, appRepo);

        var query = new SearchPlanningApplicationsQuery("user-1", "extension", AuthorityId: 42);

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert — handler no longer rejects free tier
        await Assert.That(result.Applications).HasCount().EqualTo(0);
    }
```

Remove the `using TownCrier.Application.Search;` import for `ProTierRequiredException` if it becomes unused in the test file (it was only used for the `ThrowsAsync` assertion).

- [ ] **Step 5: Run full test suite**

Run: `dotnet test api/`
Expected: All tests PASS

- [ ] **Step 6: Commit**

```bash
git add api/src/town-crier.web/Endpoints/SearchEndpoints.cs api/src/town-crier.application/Search/SearchPlanningApplicationsQueryHandler.cs api/tests/town-crier.application.tests/Search/SearchPlanningApplicationsQueryHandlerTests.cs
git commit -m "feat: migrate search endpoint to entitlement filter"
```

---

## Task 5: Gate the Notification Preferences Endpoint (Original Bug Fix)

**Files:**
- Modify: `api/src/town-crier.web/Endpoints/UserProfileEndpoints.cs`

This is the bug that prompted the entire design — PATCH `/me` allows free users to set `emailInstantEnabled = true`.

- [ ] **Step 1: Write a failing web test**

Add to `api/tests/town-crier.web.tests/Entitlements/EntitlementEndpointFilterTests.cs`:

```csharp
    [Test]
    public async Task Should_Return403_When_FreeTierEnablesInstantEmails()
    {
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        // Create the user profile first
        var token = TestJwtToken.Generate(
            userId: "auth0|free-user",
            claims: [new Claim("subscription_tier", "Free")]);
        client.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", token);

        await client.PostAsync("/v1/me", null);

        // Try to enable instant emails
        var content = new StringContent(
            """{"pushEnabled":true,"emailInstantEnabled":true}""",
            System.Text.Encoding.UTF8,
            "application/json");
        var response = await client.PatchAsync("/v1/me", content);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.Forbidden);
    }

    [Test]
    public async Task Should_AllowInstantEmails_When_PersonalTier()
    {
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        var token = TestJwtToken.Generate(
            userId: "auth0|personal-user",
            claims: [new Claim("subscription_tier", "Personal")]);
        client.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", token);

        await client.PostAsync("/v1/me", null);

        var content = new StringContent(
            """{"pushEnabled":true,"emailInstantEnabled":true}""",
            System.Text.Encoding.UTF8,
            "application/json");
        var response = await client.PatchAsync("/v1/me", content);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);
    }
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `dotnet test api/tests/town-crier.web.tests/ --filter "InstantEmails"`
Expected: `Should_Return403_When_FreeTierEnablesInstantEmails` FAILS (currently returns 200)

- [ ] **Step 3: Add entitlement gate to PATCH /me**

The tricky part: PATCH `/me` handles multiple preferences, but only `emailInstantEnabled` requires an entitlement. We need to check entitlement only when the user is trying to enable instant emails.

Rather than gating the entire endpoint with the attribute (which would block free users from updating any preference), add an inline check in the endpoint that uses `EntitlementMap`:

In `api/src/town-crier.web/Endpoints/UserProfileEndpoints.cs`, modify the PATCH `/me` handler (lines 33-56):

```csharp
        group.MapPatch("/me", async (
            ClaimsPrincipal user,
            UpdateUserProfileCommand command,
            UpdateUserProfileCommandHandler handler,
            CancellationToken ct) =>
        {
            // Check entitlement if user is trying to enable instant emails
            if (command.EmailInstantEnabled)
            {
                var tierClaim = user.FindFirst("subscription_tier")?.Value;
                var tier = Enum.TryParse<SubscriptionTier>(tierClaim, ignoreCase: true, out var parsed)
                    ? parsed
                    : SubscriptionTier.Free;

                if (!EntitlementMap.EntitlementsFor(tier).Contains(Entitlement.InstantEmails))
                {
                    return Results.Json(
                        new EntitlementErrorResponse(
                            "insufficient_entitlement",
                            Entitlement.InstantEmails.ToString(),
                            "This feature requires a paid subscription."),
                        AppJsonSerializerContext.Default.EntitlementErrorResponse,
                        statusCode: 403);
                }
            }

            var userId = user.FindFirstValue("sub")!;
            var profileCommand = new UpdateUserProfileCommand(
                userId,
                command.PushEnabled,
                command.DigestDay,
                command.EmailDigestEnabled,
                command.EmailInstantEnabled);

            try
            {
                var result = await handler.HandleAsync(profileCommand, ct).ConfigureAwait(false);
                return Results.Ok(result);
            }
            catch (UserProfileNotFoundException)
            {
                return Results.NotFound();
            }
        });
```

Add these usings at the top of the file:

```csharp
using TownCrier.Domain.Entitlements;
using TownCrier.Web.Endpoints;  // for EntitlementErrorResponse (if not already in scope)
```

Note: `EntitlementErrorResponse` and `RequiresEntitlementAttribute` are in the same namespace as this file (`TownCrier.Web.Endpoints`), so they should be accessible without an extra using.

- [ ] **Step 4: Run the tests**

Run: `dotnet test api/tests/town-crier.web.tests/ --filter "InstantEmails"`
Expected: Both tests PASS

- [ ] **Step 5: Run full test suite**

Run: `dotnet test api/`
Expected: All tests PASS

- [ ] **Step 6: Commit**

```bash
git add api/src/town-crier.web/Endpoints/UserProfileEndpoints.cs api/tests/town-crier.web.tests/Entitlements/EntitlementEndpointFilterTests.cs
git commit -m "fix: gate instant emails behind entitlement check on PATCH /me"
```

---

## Task 6: Update Background Jobs to Use EntitlementMap

**Files:**
- Modify: `api/src/town-crier.application/Notifications/DispatchNotificationCommandHandler.cs`
- Modify: `api/src/town-crier.application/Notifications/GenerateWeeklyDigestsCommandHandler.cs`
- Modify: `api/tests/town-crier.application.tests/Notifications/DispatchNotificationCommandHandlerTests.cs` (verify existing tests still pass)

- [ ] **Step 1: Run existing notification tests as baseline**

Run: `dotnet test api/tests/town-crier.application.tests/ --filter "DispatchNotification|GenerateWeeklyDigests"`
Expected: All tests PASS — this is our safety net

- [ ] **Step 2: Update DispatchNotificationCommandHandler**

In `api/src/town-crier.application/Notifications/DispatchNotificationCommandHandler.cs`, replace the ad-hoc tier check on lines 116-123:

Old code:
```csharp
        // Send instant email notification for paid tiers
        if (profile.Tier != SubscriptionTier.Free
            && profile.NotificationPreferences.EmailInstantEnabled
            && !string.IsNullOrEmpty(profile.Email))
        {
            await this.emailSender.SendNotificationAsync(profile.Email, notification, ct)
                .ConfigureAwait(false);
        }
```

New code:
```csharp
        // Send instant email notification for entitled tiers
        var entitlements = EntitlementMap.EntitlementsFor(profile.Tier);
        if (entitlements.Contains(Entitlement.InstantEmails)
            && profile.NotificationPreferences.EmailInstantEnabled
            && !string.IsNullOrEmpty(profile.Email))
        {
            await this.emailSender.SendNotificationAsync(profile.Email, notification, ct)
                .ConfigureAwait(false);
        }
```

Add the using at the top:
```csharp
using TownCrier.Domain.Entitlements;
```

- [ ] **Step 3: Update GenerateWeeklyDigestsCommandHandler**

In `api/src/town-crier.application/Notifications/GenerateWeeklyDigestsCommandHandler.cs`, replace the ad-hoc tier check on lines 47-48:

Old code:
```csharp
            var wantsPush = profile.Tier == SubscriptionTier.Pro
                && profile.NotificationPreferences.PushEnabled;
```

New code (add a `PushDigest` entitlement for this — but wait, we didn't define one in the enum. The weekly push digest being Pro-only is a tier gate. For now, leave this as-is — the push digest doesn't map cleanly to an entitlement we defined. If we want to bring it under the framework, we'd add an `Entitlement.PushDigest` value. Since this is a background job with no endpoint, the ad-hoc check is less risky. Leave it for a future task).

Actually, keep this handler unchanged for now. The spec says "gradually migrate existing ad-hoc checks."

- [ ] **Step 4: Run notification tests again**

Run: `dotnet test api/tests/town-crier.application.tests/ --filter "DispatchNotification|GenerateWeeklyDigests"`
Expected: All tests PASS — the behaviour is identical since `EntitlementMap.EntitlementsFor(Free)` returns no entitlements (same as the old `!= Free` check)

- [ ] **Step 5: Run full test suite**

Run: `dotnet test api/`
Expected: All tests PASS

- [ ] **Step 6: Commit**

```bash
git add api/src/town-crier.application/Notifications/DispatchNotificationCommandHandler.cs
git commit -m "refactor: use EntitlementMap for instant email check in dispatch handler"
```

---

## Task 7: Auth0 Management Client Port and Adapter

**Files:**
- Create: `api/src/town-crier.application/Auth/IAuth0ManagementClient.cs`
- Create: `api/src/town-crier.infrastructure/Auth/Auth0ManagementClient.cs`
- Create: `api/src/town-crier.infrastructure/Auth/NoOpAuth0ManagementClient.cs`
- Modify: `api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs`

- [ ] **Step 1: Create the port (interface)**

```csharp
// api/src/town-crier.application/Auth/IAuth0ManagementClient.cs
namespace TownCrier.Application.Auth;

public interface IAuth0ManagementClient
{
    Task UpdateSubscriptionTierAsync(string userId, string tier, CancellationToken ct);
}
```

- [ ] **Step 2: Create the NoOp adapter** (for dev/test environments without Auth0 M2M credentials)

```csharp
// api/src/town-crier.infrastructure/Auth/NoOpAuth0ManagementClient.cs
using TownCrier.Application.Auth;

namespace TownCrier.Infrastructure.Auth;

public sealed class NoOpAuth0ManagementClient : IAuth0ManagementClient
{
    public Task UpdateSubscriptionTierAsync(string userId, string tier, CancellationToken ct)
    {
        return Task.CompletedTask;
    }
}
```

- [ ] **Step 3: Create the real adapter**

This calls the Auth0 Management API using client credentials. It obtains an M2M token and PATCHes user `app_metadata`.

```csharp
// api/src/town-crier.infrastructure/Auth/Auth0ManagementClient.cs
using System.Net.Http.Json;
using System.Text.Json;
using System.Text.Json.Serialization;
using TownCrier.Application.Auth;

namespace TownCrier.Infrastructure.Auth;

public sealed class Auth0ManagementClient : IAuth0ManagementClient
{
    private readonly HttpClient httpClient;
    private readonly string domain;
    private readonly string clientId;
    private readonly string clientSecret;
    private string? cachedToken;
    private DateTimeOffset tokenExpiry;

    public Auth0ManagementClient(HttpClient httpClient, string domain, string clientId, string clientSecret)
    {
        this.httpClient = httpClient;
        this.domain = domain;
        this.clientId = clientId;
        this.clientSecret = clientSecret;
    }

    public async Task UpdateSubscriptionTierAsync(string userId, string tier, CancellationToken ct)
    {
        var token = await this.GetTokenAsync(ct).ConfigureAwait(false);

        using var request = new HttpRequestMessage(HttpMethod.Patch,
            $"https://{this.domain}/api/v2/users/{Uri.EscapeDataString(userId)}");
        request.Headers.Authorization = new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", token);
        request.Content = JsonContent.Create(
            new { app_metadata = new { subscription_tier = tier } });

        using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);
        response.EnsureSuccessStatusCode();
    }

    private async Task<string> GetTokenAsync(CancellationToken ct)
    {
        if (this.cachedToken is not null && DateTimeOffset.UtcNow < this.tokenExpiry)
        {
            return this.cachedToken;
        }

        using var request = new HttpRequestMessage(HttpMethod.Post, $"https://{this.domain}/oauth/token");
        request.Content = JsonContent.Create(new
        {
            grant_type = "client_credentials",
            client_id = this.clientId,
            client_secret = this.clientSecret,
            audience = $"https://{this.domain}/api/v2/",
        });

        using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);
        response.EnsureSuccessStatusCode();

        var tokenResponse = await response.Content.ReadFromJsonAsync(
            Auth0TokenResponseContext.Default.Auth0TokenResponse, ct).ConfigureAwait(false);

        this.cachedToken = tokenResponse!.AccessToken;
        this.tokenExpiry = DateTimeOffset.UtcNow.AddSeconds(tokenResponse.ExpiresIn - 60);
        return this.cachedToken;
    }
}

internal sealed record Auth0TokenResponse(
    [property: JsonPropertyName("access_token")] string AccessToken,
    [property: JsonPropertyName("expires_in")] int ExpiresIn);

[JsonSerializable(typeof(Auth0TokenResponse))]
internal sealed partial class Auth0TokenResponseContext : JsonSerializerContext;
```

- [ ] **Step 4: Register in DI**

In `api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs`, add to `AddInfrastructureServices` after the email sender registration (after line 64):

```csharp
        var auth0M2mClientId = configuration["Auth0:M2M:ClientId"];
        var auth0M2mClientSecret = configuration["Auth0:M2M:ClientSecret"];
        var auth0Domain = configuration["Auth0:Domain"];
        if (!string.IsNullOrEmpty(auth0M2mClientId) && !string.IsNullOrEmpty(auth0M2mClientSecret) && !string.IsNullOrEmpty(auth0Domain))
        {
            services.AddHttpClient<IAuth0ManagementClient, Auth0ManagementClient>((httpClient, sp) =>
                new Auth0ManagementClient(httpClient, auth0Domain, auth0M2mClientId, auth0M2mClientSecret));
        }
        else
        {
            services.AddSingleton<IAuth0ManagementClient, NoOpAuth0ManagementClient>();
        }
```

Add the usings:
```csharp
using TownCrier.Application.Auth;
using TownCrier.Infrastructure.Auth;
```

- [ ] **Step 5: Verify build**

Run: `dotnet build api/`
Expected: Build succeeded

- [ ] **Step 6: Run full test suite**

Run: `dotnet test api/`
Expected: All tests PASS (tests use NoOp since no M2M config is set)

- [ ] **Step 7: Commit**

```bash
git add api/src/town-crier.application/Auth/ api/src/town-crier.infrastructure/Auth/ api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs
git commit -m "feat: add Auth0 Management Client port and adapter for tier sync"
```

---

## Task 8: Sync Tier to Auth0 on Grant Subscription

**Files:**
- Modify: `api/src/town-crier.application/Admin/GrantSubscriptionCommandHandler.cs`
- Create: `api/tests/town-crier.application.tests/Admin/GrantSubscriptionCommandHandlerTests.cs`
- Create: `api/tests/town-crier.application.tests/Admin/FakeAuth0ManagementClient.cs`

- [ ] **Step 1: Create the fake Auth0 client for tests**

```csharp
// api/tests/town-crier.application.tests/Admin/FakeAuth0ManagementClient.cs
using TownCrier.Application.Auth;

namespace TownCrier.Application.Tests.Admin;

internal sealed class FakeAuth0ManagementClient : IAuth0ManagementClient
{
    public List<(string UserId, string Tier)> Updates { get; } = [];

    public Task UpdateSubscriptionTierAsync(string userId, string tier, CancellationToken ct)
    {
        this.Updates.Add((userId, tier));
        return Task.CompletedTask;
    }
}
```

- [ ] **Step 2: Write the failing tests**

```csharp
// api/tests/town-crier.application.tests/Admin/GrantSubscriptionCommandHandlerTests.cs
using TownCrier.Application.Admin;
using TownCrier.Application.Tests.Notifications;
using TownCrier.Application.Tests.UserProfiles;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.Admin;

public sealed class GrantSubscriptionCommandHandlerTests
{
    [Test]
    public async Task Should_SyncTierToAuth0_When_GrantingProSubscription()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var auth0Client = new FakeAuth0ManagementClient();
        var profile = new UserProfileBuilder()
            .WithUserId("auth0|user-1")
            .WithEmail("user@example.com")
            .Build();
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new GrantSubscriptionCommandHandler(repository, auth0Client);
        var command = new GrantSubscriptionCommand("user@example.com", SubscriptionTier.Pro);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(auth0Client.Updates).HasCount().EqualTo(1);
        await Assert.That(auth0Client.Updates[0].UserId).IsEqualTo("auth0|user-1");
        await Assert.That(auth0Client.Updates[0].Tier).IsEqualTo("Pro");
    }

    [Test]
    public async Task Should_SyncTierToAuth0_When_DowngradingToFree()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var auth0Client = new FakeAuth0ManagementClient();
        var profile = new UserProfileBuilder()
            .WithUserId("auth0|user-1")
            .WithEmail("user@example.com")
            .WithTier(SubscriptionTier.Pro)
            .Build();
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new GrantSubscriptionCommandHandler(repository, auth0Client);
        var command = new GrantSubscriptionCommand("user@example.com", SubscriptionTier.Free);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(auth0Client.Updates).HasCount().EqualTo(1);
        await Assert.That(auth0Client.Updates[0].Tier).IsEqualTo("Free");
    }
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `dotnet test api/tests/town-crier.application.tests/ --filter "GrantSubscription"`
Expected: FAIL — `GrantSubscriptionCommandHandler` constructor doesn't accept `IAuth0ManagementClient` yet

- [ ] **Step 4: Update the handler to sync to Auth0**

Replace `api/src/town-crier.application/Admin/GrantSubscriptionCommandHandler.cs`:

```csharp
using TownCrier.Application.Auth;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Admin;

public sealed class GrantSubscriptionCommandHandler
{
    private static readonly DateTimeOffset FarFutureExpiry = new(2099, 12, 31, 0, 0, 0, TimeSpan.Zero);

    private readonly IUserProfileRepository repository;
    private readonly IAuth0ManagementClient auth0Client;

    public GrantSubscriptionCommandHandler(IUserProfileRepository repository, IAuth0ManagementClient auth0Client)
    {
        this.repository = repository;
        this.auth0Client = auth0Client;
    }

    public async Task<GrantSubscriptionResult> HandleAsync(GrantSubscriptionCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var profile = await this.repository.GetByEmailAsync(command.Email, ct).ConfigureAwait(false)
            ?? throw new UserProfileNotFoundException($"No user profile found for email '{command.Email}'.");

        if (command.Tier == SubscriptionTier.Free)
        {
            profile.ExpireSubscription();
        }
        else
        {
            profile.ActivateSubscription(command.Tier, FarFutureExpiry);
        }

        await this.repository.SaveAsync(profile, ct).ConfigureAwait(false);
        await this.auth0Client.UpdateSubscriptionTierAsync(profile.UserId, profile.Tier.ToString(), ct)
            .ConfigureAwait(false);

        return new GrantSubscriptionResult(profile.UserId, profile.Email, profile.Tier);
    }
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `dotnet test api/tests/town-crier.application.tests/ --filter "GrantSubscription"`
Expected: All tests PASS

- [ ] **Step 6: Run full test suite**

Run: `dotnet test api/`
Expected: All tests PASS

- [ ] **Step 7: Commit**

```bash
git add api/src/town-crier.application/Admin/GrantSubscriptionCommandHandler.cs api/tests/town-crier.application.tests/Admin/
git commit -m "feat: sync subscription tier to Auth0 app_metadata on grant"
```

---

## Task 9: Deploy Auth0 Post-Login Action

**Files:**
- None in codebase — this is an Auth0 configuration step

This task deploys the Post-Login Action to Auth0 using the `auth0` CLI.

- [ ] **Step 1: Create the Action**

```bash
auth0 actions create \
  --name "Add Subscription Tier Claim" \
  --trigger post-login \
  --code "exports.onExecutePostLogin = async (event, api) => {
  const tier = event.user.app_metadata?.subscription_tier || 'Free';
  api.accessToken.setCustomClaim('subscription_tier', tier);
};"
```

- [ ] **Step 2: Deploy the Action**

```bash
auth0 actions deploy <action-id-from-step-1>
```

- [ ] **Step 3: Add the Action to the Login Flow**

```bash
auth0 api patch "actions/triggers/post-login/bindings" --data '{
  "bindings": [{"ref": {"type": "action_id", "value": "<action-id>"}, "display_name": "Add Subscription Tier Claim"}]
}'
```

- [ ] **Step 4: Verify by inspecting a token**

Log in via the web app or iOS app, decode the access token at jwt.io, and verify `subscription_tier` claim is present (should be `Free` for a user without `app_metadata.subscription_tier` set).

- [ ] **Step 5: Commit** (nothing to commit — this is Auth0 config, not code)

---

## Task 10: Create Auth0 M2M Application

**Files:**
- None in codebase — this is Auth0 + Azure configuration

- [ ] **Step 1: Create the M2M application in Auth0**

```bash
auth0 apps create \
  --name "Town Crier API (M2M)" \
  --type m2m \
  --auth-method client-secret-post
```

Note the client ID and client secret from the output.

- [ ] **Step 2: Grant the M2M app `update:users` scope on the Management API**

```bash
auth0 api post "client-grants" --data '{
  "client_id": "<m2m-client-id>",
  "audience": "https://towncrierapp.uk.auth0.com/api/v2/",
  "scope": ["update:users"]
}'
```

- [ ] **Step 3: Configure the API with M2M credentials**

Add to the appropriate Azure Container Apps environment variables (via Pulumi config or `az` CLI):

```
Auth0__M2M__ClientId=<client-id>
Auth0__M2M__ClientSecret=<client-secret>
```

For local development, add to `appsettings.Development.json` or user secrets.

- [ ] **Step 4: Test end-to-end**

Use the admin endpoint to grant a subscription, then verify the user's Auth0 `app_metadata` was updated:

```bash
auth0 users show <user-id> --json | jq '.app_metadata'
```

---

## Verification Checklist

After all tasks are complete:

- [ ] `dotnet build api/` — succeeds
- [ ] `dotnet test api/` — all tests pass
- [ ] `dotnet format api/ --verify-no-changes` — no formatting issues
- [ ] Free user calling GET `/v1/search` → 403 with `insufficient_entitlement` error
- [ ] Pro user calling GET `/v1/search` → 200
- [ ] Free user PATCH `/v1/me` with `emailInstantEnabled: true` → 403
- [ ] Personal user PATCH `/v1/me` with `emailInstantEnabled: true` → 200
- [ ] Admin grants Pro → Auth0 `app_metadata.subscription_tier` updated
- [ ] JWT token includes `subscription_tier` claim after login
