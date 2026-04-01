# Manual Subscription Grants Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an admin API endpoint that sets a user's subscription tier by email address, secured with a shared API key, so subscriptions can be manually granted during the build phase.

**Architecture:** New CQRS command/handler for the grant operation. Email field added to UserProfile domain model and Cosmos document so the admin endpoint can look up users by email without Auth0 Management API. API key authentication via an endpoint filter scoped to `/v1/admin/*` routes.

**Tech Stack:** .NET 10, ASP.NET Core Minimal APIs, Cosmos DB SDK (via CosmosRestClient), TUnit

---

### Task 1: Add `Personal` to `SubscriptionTier` enum

The enum currently only has `Free` and `Pro`. The spec requires granting `Personal` tier.

**Files:**
- Modify: `api/src/town-crier.domain/UserProfiles/SubscriptionTier.cs`

- [ ] **Step 1: Add `Personal` to the enum**

```csharp
public enum SubscriptionTier
{
    Free,
    Personal,
    Pro,
}
```

- [ ] **Step 2: Run all tests to verify nothing breaks**

Run: `dotnet test api/tests/town-crier.application.tests && dotnet test api/tests/town-crier.infrastructure.tests`
Expected: All tests PASS. The existing code uses `SubscriptionTier.Free` and `SubscriptionTier.Pro` explicitly — adding a new member between them doesn't break anything. The Cosmos document stores tier as a string (`"Free"`, `"Pro"`) not an integer, so the enum ordering is irrelevant to serialization.

- [ ] **Step 3: Commit**

```bash
git add api/src/town-crier.domain/UserProfiles/SubscriptionTier.cs
git commit -m "feat(domain): add Personal tier to SubscriptionTier enum"
```

---

### Task 2: Add `Email` to `UserProfile` domain model

The admin endpoint looks up users by email. Email needs to live on the domain model and be populated at registration time.

**Files:**
- Modify: `api/src/town-crier.domain/UserProfiles/UserProfile.cs`

- [ ] **Step 1: Add `Email` property and update constructor, `Register`, and `Reconstitute`**

In `UserProfile.cs`, add `Email` as a nullable string property. The constructor takes it, `Register` accepts it as a parameter, and `Reconstitute` passes it through.

Add `email` parameter to the private constructor (after `userId`):

```csharp
private UserProfile(
    string userId,
    string? email,
    string? postcode,
    NotificationPreferences notificationPreferences,
    SubscriptionTier tier,
    DateTimeOffset? subscriptionExpiry,
    string? originalTransactionId,
    DateTimeOffset? gracePeriodExpiry)
{
    this.UserId = userId;
    this.Email = email;
    this.Postcode = postcode;
    this.NotificationPreferences = notificationPreferences;
    this.Tier = tier;
    this.SubscriptionExpiry = subscriptionExpiry;
    this.OriginalTransactionId = originalTransactionId;
    this.GracePeriodExpiry = gracePeriodExpiry;
}
```

Add the property after `UserId`:

```csharp
public string? Email { get; private set; }
```

Update `Register` to accept `email` parameter:

```csharp
public static UserProfile Register(string userId, string? email = null)
{
    ArgumentException.ThrowIfNullOrWhiteSpace(userId);

    return new UserProfile(
        userId,
        email,
        postcode: null,
        notificationPreferences: NotificationPreferences.Default,
        tier: SubscriptionTier.Free,
        subscriptionExpiry: null,
        originalTransactionId: null,
        gracePeriodExpiry: null);
}
```

Update `Reconstitute` to accept `email` parameter (add it after `userId`):

```csharp
internal static UserProfile Reconstitute(
    string userId,
    string? email,
    string? postcode,
    NotificationPreferences notificationPreferences,
    Dictionary<string, ZoneNotificationPreferences> zonePreferences,
    SubscriptionTier tier,
    DateTimeOffset? subscriptionExpiry,
    string? originalTransactionId,
    DateTimeOffset? gracePeriodExpiry)
{
    var profile = new UserProfile(
        userId,
        email,
        postcode,
        notificationPreferences,
        tier,
        subscriptionExpiry,
        originalTransactionId,
        gracePeriodExpiry);

    foreach (var (zoneId, prefs) in zonePreferences)
    {
        profile.zonePreferences[zoneId] = prefs;
    }

    return profile;
}
```

- [ ] **Step 2: Run tests — expect compilation failures**

Run: `dotnet build api`
Expected: FAIL — `Reconstitute` callers in `UserProfileDocument.ToDomain()` and the `UserProfileBuilder` test helper need updating. This confirms the change propagated correctly.

- [ ] **Step 3: Commit**

```bash
git add api/src/town-crier.domain/UserProfiles/UserProfile.cs
git commit -m "feat(domain): add Email property to UserProfile"
```

---

### Task 3: Update `UserProfileDocument` for email

Wire the email field through the Cosmos document so it persists and round-trips.

**Files:**
- Modify: `api/src/town-crier.infrastructure/UserProfiles/UserProfileDocument.cs`
- Modify: `api/tests/town-crier.infrastructure.tests/UserProfiles/UserProfileDocumentTests.cs`

- [ ] **Step 1: Add `Email` field to `UserProfileDocument`**

Add after the `UserId` property:

```csharp
public string? Email { get; init; }
```

Update `FromDomain`:

```csharp
public static UserProfileDocument FromDomain(UserProfile profile)
{
    ArgumentNullException.ThrowIfNull(profile);

    return new UserProfileDocument
    {
        Id = profile.UserId,
        UserId = profile.UserId,
        Email = profile.Email,
        Postcode = profile.Postcode,
        PushEnabled = profile.NotificationPreferences.PushEnabled,
        DigestDay = profile.NotificationPreferences.DigestDay,
        ZonePreferences = new Dictionary<string, ZoneNotificationPreferences>(profile.AllZonePreferences),
        Tier = profile.Tier.ToString(),
        SubscriptionExpiry = profile.SubscriptionExpiry,
        OriginalTransactionId = profile.OriginalTransactionId,
        GracePeriodExpiry = profile.GracePeriodExpiry,
    };
}
```

Update `ToDomain`:

```csharp
public UserProfile ToDomain()
{
    var tier = Enum.Parse<SubscriptionTier>(this.Tier);
    var notificationPreferences = new NotificationPreferences(this.PushEnabled, this.DigestDay);

    return UserProfile.Reconstitute(
        this.UserId,
        this.Email,
        this.Postcode,
        notificationPreferences,
        this.ZonePreferences,
        tier,
        this.SubscriptionExpiry,
        this.OriginalTransactionId,
        this.GracePeriodExpiry);
}
```

- [ ] **Step 2: Update `UserProfileBuilder` in tests**

In `api/tests/town-crier.application.tests/Notifications/UserProfileBuilder.cs`, the `Build()` method calls `UserProfile.Register(this.userId)` which now accepts an optional email. No change needed since email defaults to `null`. Verify it compiles.

- [ ] **Step 3: Write a round-trip test for email**

Add to `UserProfileDocumentTests.cs`:

```csharp
[Test]
public async Task Should_PreserveEmail_When_RoundTripped()
{
    // Arrange
    var original = UserProfile.Register("auth0|user-1", "test@example.com");

    // Act
    var document = UserProfileDocument.FromDomain(original);
    var roundTripped = document.ToDomain();

    // Assert
    await Assert.That(document.Email).IsEqualTo("test@example.com");
    await Assert.That(roundTripped.Email).IsEqualTo("test@example.com");
}

[Test]
public async Task Should_HandleNullEmail_When_RoundTripped()
{
    // Arrange
    var original = UserProfile.Register("auth0|user-1");

    // Act
    var document = UserProfileDocument.FromDomain(original);
    var roundTripped = document.ToDomain();

    // Assert
    await Assert.That(document.Email).IsNull();
    await Assert.That(roundTripped.Email).IsNull();
}
```

- [ ] **Step 4: Run all tests**

Run: `dotnet test api/tests/town-crier.infrastructure.tests`
Expected: All tests PASS, including the two new email round-trip tests.

- [ ] **Step 5: Commit**

```bash
git add api/src/town-crier.infrastructure/UserProfiles/UserProfileDocument.cs api/tests/town-crier.infrastructure.tests/UserProfiles/UserProfileDocumentTests.cs
git commit -m "feat(infra): add Email field to UserProfileDocument with round-trip tests"
```

---

### Task 4: Add `GetByEmailAsync` to repository

The admin endpoint needs to look up a user by email. Add this method to the repository interface and implementation.

**Files:**
- Modify: `api/src/town-crier.application/UserProfiles/IUserProfileRepository.cs`
- Modify: `api/src/town-crier.infrastructure/UserProfiles/CosmosUserProfileRepository.cs`
- Modify: `api/tests/town-crier.application.tests/UserProfiles/FakeUserProfileRepository.cs`

- [ ] **Step 1: Write the failing test**

Add a new test file `api/tests/town-crier.application.tests/UserProfiles/FakeUserProfileRepositoryTests.cs`:

```csharp
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.UserProfiles;

public sealed class FakeUserProfileRepositoryTests
{
    [Test]
    public async Task GetByEmailAsync_Should_ReturnProfile_When_EmailMatches()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-1", "test@example.com");
        await repository.SaveAsync(profile, CancellationToken.None);

        // Act
        var result = await repository.GetByEmailAsync("test@example.com", CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.UserId).IsEqualTo("auth0|user-1");
    }

    [Test]
    public async Task GetByEmailAsync_Should_ReturnNull_When_NoMatch()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();

        // Act
        var result = await repository.GetByEmailAsync("nobody@example.com", CancellationToken.None);

        // Assert
        await Assert.That(result).IsNull();
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `dotnet test api/tests/town-crier.application.tests --filter "FakeUserProfileRepositoryTests"`
Expected: FAIL — `GetByEmailAsync` does not exist on `IUserProfileRepository`.

- [ ] **Step 3: Add `GetByEmailAsync` to the interface**

In `IUserProfileRepository.cs`, add:

```csharp
Task<UserProfile?> GetByEmailAsync(string email, CancellationToken ct);
```

- [ ] **Step 4: Implement in `CosmosUserProfileRepository`**

Add to `CosmosUserProfileRepository.cs`:

```csharp
public async Task<UserProfile?> GetByEmailAsync(string email, CancellationToken ct)
{
    var documents = await this.client.QueryAsync(
        CosmosContainerNames.Users,
        "SELECT * FROM c WHERE c.email = @email",
        [new QueryParameter("@email", email)],
        partitionKey: null,
        CosmosJsonSerializerContext.Default.UserProfileDocument,
        ct).ConfigureAwait(false);

    return documents.Count > 0 ? documents[0].ToDomain() : null;
}
```

- [ ] **Step 5: Implement in `FakeUserProfileRepository`**

Add to `FakeUserProfileRepository.cs`:

```csharp
public Task<UserProfile?> GetByEmailAsync(string email, CancellationToken ct)
{
    var profile = this.store.Values
        .FirstOrDefault(p => string.Equals(p.Email, email, StringComparison.OrdinalIgnoreCase));
    return Task.FromResult(profile);
}
```

- [ ] **Step 6: Run tests**

Run: `dotnet test api/tests/town-crier.application.tests --filter "FakeUserProfileRepositoryTests"`
Expected: Both tests PASS.

- [ ] **Step 7: Commit**

```bash
git add api/src/town-crier.application/UserProfiles/IUserProfileRepository.cs api/src/town-crier.infrastructure/UserProfiles/CosmosUserProfileRepository.cs api/tests/town-crier.application.tests/UserProfiles/FakeUserProfileRepository.cs api/tests/town-crier.application.tests/UserProfiles/FakeUserProfileRepositoryTests.cs
git commit -m "feat(application): add GetByEmailAsync to IUserProfileRepository"
```

---

### Task 5: Pass email through `CreateUserProfileCommand`

When a user creates their profile via `POST /v1/me`, the email from their Auth0 token should be stored on the profile.

**Files:**
- Modify: `api/src/town-crier.application/UserProfiles/CreateUserProfileCommand.cs`
- Modify: `api/src/town-crier.application/UserProfiles/CreateUserProfileCommandHandler.cs`
- Modify: `api/src/town-crier.web/Endpoints/UserProfileEndpoints.cs`
- Modify: `api/tests/town-crier.application.tests/UserProfiles/CreateUserProfileCommandHandlerTests.cs`

- [ ] **Step 1: Write the failing test**

Add to `CreateUserProfileCommandHandlerTests.cs`:

```csharp
[Test]
public async Task Should_StoreEmail_When_ProfileCreated()
{
    // Arrange
    var repository = new FakeUserProfileRepository();
    var handler = new CreateUserProfileCommandHandler(repository);
    var command = new CreateUserProfileCommand("auth0|user-email", "user@example.com");

    // Act
    await handler.HandleAsync(command, CancellationToken.None);

    // Assert
    var saved = repository.GetByUserId("auth0|user-email");
    await Assert.That(saved!.Email).IsEqualTo("user@example.com");
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `dotnet test api/tests/town-crier.application.tests --filter "Should_StoreEmail_When_ProfileCreated"`
Expected: FAIL — `CreateUserProfileCommand` constructor doesn't accept email.

- [ ] **Step 3: Add `Email` to `CreateUserProfileCommand`**

```csharp
public sealed record CreateUserProfileCommand(string UserId, string? Email = null);
```

- [ ] **Step 4: Update `CreateUserProfileCommandHandler` to pass email to `Register`**

```csharp
public async Task<CreateUserProfileResult> HandleAsync(CreateUserProfileCommand command, CancellationToken ct)
{
    ArgumentNullException.ThrowIfNull(command);

    var existing = await this.repository.GetByUserIdAsync(command.UserId, ct).ConfigureAwait(false);
    if (existing is not null)
    {
        return new CreateUserProfileResult(
            existing.UserId,
            existing.Postcode,
            existing.NotificationPreferences.PushEnabled,
            existing.Tier);
    }

    var profile = UserProfile.Register(command.UserId, command.Email);
    await this.repository.SaveAsync(profile, ct).ConfigureAwait(false);

    return new CreateUserProfileResult(
        profile.UserId,
        profile.Postcode,
        profile.NotificationPreferences.PushEnabled,
        profile.Tier);
}
```

- [ ] **Step 5: Update `UserProfileEndpoints.cs` to extract email from claims**

In the `MapPost("/me", ...)` handler, extract the email claim and pass it to the command:

```csharp
group.MapPost("/me", async (
    ClaimsPrincipal user,
    CreateUserProfileCommandHandler handler,
    CancellationToken ct) =>
{
    var userId = user.FindFirstValue("sub")!;
    var email = user.FindFirstValue("email");
    var result = await handler.HandleAsync(new CreateUserProfileCommand(userId, email), ct).ConfigureAwait(false);
    return Results.Ok(result);
});
```

Note: Auth0 JWTs include the `email` claim when the user has a verified email. The claim name is `email` (not namespaced) because `MapInboundClaims = false` is set in the JWT configuration, which preserves the original claim names from the token.

- [ ] **Step 6: Run tests**

Run: `dotnet test api/tests/town-crier.application.tests --filter "CreateUserProfileCommandHandler"`
Expected: All tests PASS including the new email test. Existing tests still pass because `Email` defaults to `null`.

- [ ] **Step 7: Commit**

```bash
git add api/src/town-crier.application/UserProfiles/CreateUserProfileCommand.cs api/src/town-crier.application/UserProfiles/CreateUserProfileCommandHandler.cs api/src/town-crier.web/Endpoints/UserProfileEndpoints.cs api/tests/town-crier.application.tests/UserProfiles/CreateUserProfileCommandHandlerTests.cs
git commit -m "feat(application): store email from Auth0 claims on profile creation"
```

---

### Task 6: Create `GrantSubscriptionCommand` and handler

The core business logic: look up a user by email, set their tier.

**Files:**
- Create: `api/src/town-crier.application/Admin/GrantSubscriptionCommand.cs`
- Create: `api/src/town-crier.application/Admin/GrantSubscriptionCommandHandler.cs`
- Create: `api/src/town-crier.application/Admin/GrantSubscriptionResult.cs`
- Create: `api/tests/town-crier.application.tests/Admin/GrantSubscriptionCommandHandlerTests.cs`

- [ ] **Step 1: Write the failing tests**

Create `api/tests/town-crier.application.tests/Admin/GrantSubscriptionCommandHandlerTests.cs`:

```csharp
using TownCrier.Application.Admin;
using TownCrier.Application.Tests.UserProfiles;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.Admin;

public sealed class GrantSubscriptionCommandHandlerTests
{
    [Test]
    public async Task Should_ActivateProTier_When_UserFoundByEmail()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-1", "friend@example.com");
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new GrantSubscriptionCommandHandler(repository);
        var command = new GrantSubscriptionCommand("friend@example.com", SubscriptionTier.Pro);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Pro);
        await Assert.That(result.Email).IsEqualTo("friend@example.com");
    }

    [Test]
    public async Task Should_ActivatePersonalTier_When_Requested()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-1", "friend@example.com");
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new GrantSubscriptionCommandHandler(repository);
        var command = new GrantSubscriptionCommand("friend@example.com", SubscriptionTier.Personal);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Personal);
    }

    [Test]
    public async Task Should_RevokeToFree_When_FreeTierRequested()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-1", "friend@example.com");
        profile.ActivateSubscription(SubscriptionTier.Pro, DateTimeOffset.UtcNow.AddYears(73));
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new GrantSubscriptionCommandHandler(repository);
        var command = new GrantSubscriptionCommand("friend@example.com", SubscriptionTier.Free);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Free);
    }

    [Test]
    public async Task Should_PersistTierChange_When_Granted()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-1", "friend@example.com");
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new GrantSubscriptionCommandHandler(repository);
        var command = new GrantSubscriptionCommand("friend@example.com", SubscriptionTier.Pro);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var saved = repository.GetByUserId("auth0|user-1");
        await Assert.That(saved!.Tier).IsEqualTo(SubscriptionTier.Pro);
    }

    [Test]
    public async Task Should_ThrowUserProfileNotFoundException_When_EmailNotFound()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var handler = new GrantSubscriptionCommandHandler(repository);
        var command = new GrantSubscriptionCommand("nobody@example.com", SubscriptionTier.Pro);

        // Act & Assert
        await Assert.ThrowsAsync<UserProfileNotFoundException>(
            () => handler.HandleAsync(command, CancellationToken.None));
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `dotnet test api/tests/town-crier.application.tests --filter "GrantSubscriptionCommandHandlerTests"`
Expected: FAIL — types don't exist yet.

- [ ] **Step 3: Create `GrantSubscriptionCommand`**

Create `api/src/town-crier.application/Admin/GrantSubscriptionCommand.cs`:

```csharp
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Admin;

public sealed record GrantSubscriptionCommand(string Email, SubscriptionTier Tier);
```

- [ ] **Step 4: Create `GrantSubscriptionResult`**

Create `api/src/town-crier.application/Admin/GrantSubscriptionResult.cs`:

```csharp
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Admin;

public sealed record GrantSubscriptionResult(
    string UserId,
    string? Email,
    SubscriptionTier Tier);
```

- [ ] **Step 5: Create `GrantSubscriptionCommandHandler`**

Create `api/src/town-crier.application/Admin/GrantSubscriptionCommandHandler.cs`:

```csharp
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Admin;

public sealed class GrantSubscriptionCommandHandler
{
    private static readonly DateTimeOffset FarFutureExpiry = new(2099, 12, 31, 0, 0, 0, TimeSpan.Zero);

    private readonly IUserProfileRepository repository;

    public GrantSubscriptionCommandHandler(IUserProfileRepository repository)
    {
        this.repository = repository;
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

        return new GrantSubscriptionResult(profile.UserId, profile.Email, profile.Tier);
    }
}
```

- [ ] **Step 6: Run tests**

Run: `dotnet test api/tests/town-crier.application.tests --filter "GrantSubscriptionCommandHandlerTests"`
Expected: All 5 tests PASS.

- [ ] **Step 7: Commit**

```bash
git add api/src/town-crier.application/Admin/ api/tests/town-crier.application.tests/Admin/
git commit -m "feat(application): add GrantSubscriptionCommandHandler with tests"
```

---

### Task 7: Add API key endpoint filter

Create a reusable endpoint filter that checks the `X-Admin-Key` header against a configured secret. This filter will be applied to the admin route group.

**Files:**
- Create: `api/src/town-crier.web/Endpoints/AdminApiKeyFilter.cs`

- [ ] **Step 1: Create the endpoint filter**

Create `api/src/town-crier.web/Endpoints/AdminApiKeyFilter.cs`:

```csharp
namespace TownCrier.Web.Endpoints;

internal sealed class AdminApiKeyFilter : IEndpointFilter
{
    private const string ApiKeyHeaderName = "X-Admin-Key";

    private readonly string expectedApiKey;

    public AdminApiKeyFilter(IConfiguration configuration)
    {
        this.expectedApiKey = configuration["Admin:ApiKey"]
            ?? throw new InvalidOperationException("Admin:ApiKey configuration is required.");
    }

    public async ValueTask<object?> InvokeAsync(
        EndpointFilterInvocationContext context,
        EndpointFilterDelegate next)
    {
        if (!context.HttpContext.Request.Headers.TryGetValue(ApiKeyHeaderName, out var providedKey)
            || !string.Equals(this.expectedApiKey, providedKey.ToString(), StringComparison.Ordinal))
        {
            return Results.Unauthorized();
        }

        return await next(context).ConfigureAwait(false);
    }
}
```

- [ ] **Step 2: Commit**

```bash
git add api/src/town-crier.web/Endpoints/AdminApiKeyFilter.cs
git commit -m "feat(web): add AdminApiKeyFilter for X-Admin-Key header auth"
```

---

### Task 8: Wire up admin endpoint, DI, and JSON serialization

Register the handler, create the admin endpoint group, and register the new types with the AOT-compatible JSON serializer context.

**Files:**
- Create: `api/src/town-crier.web/Endpoints/AdminEndpoints.cs`
- Modify: `api/src/town-crier.web/Extensions/WebApplicationExtensions.cs`
- Modify: `api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs`
- Modify: `api/src/town-crier.web/AppJsonSerializerContext.cs`

- [ ] **Step 1: Create `AdminEndpoints.cs`**

Create `api/src/town-crier.web/Endpoints/AdminEndpoints.cs`:

```csharp
using TownCrier.Application.Admin;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Web.Endpoints;

internal static class AdminEndpoints
{
    public static void MapAdminEndpoints(this RouteGroupBuilder group)
    {
        var admin = group.MapGroup("/admin")
            .AddEndpointFilter<AdminApiKeyFilter>()
            .AllowAnonymous();

        admin.MapPut("/subscriptions", async (
            GrantSubscriptionCommand command,
            GrantSubscriptionCommandHandler handler,
            CancellationToken ct) =>
        {
            try
            {
                var result = await handler.HandleAsync(command, ct).ConfigureAwait(false);
                return Results.Ok(result);
            }
            catch (UserProfileNotFoundException)
            {
                return Results.NotFound();
            }
        });
    }
}
```

Note: `.AllowAnonymous()` bypasses JWT auth for the admin group — authentication is handled entirely by the `AdminApiKeyFilter`. This prevents needing a valid Auth0 token when using curl.

- [ ] **Step 2: Register handler in DI**

In `ServiceCollectionExtensions.cs`, add to the `AddApplicationServices` method, after the existing `GetDemoAccountQueryHandler` registration:

```csharp
services.AddTransient<GrantSubscriptionCommandHandler>();
```

Add the using statement at the top of the file:

```csharp
using TownCrier.Application.Admin;
```

- [ ] **Step 3: Map admin endpoints**

In `WebApplicationExtensions.cs`, add after `v1.MapDemoAccountEndpoints();`:

```csharp
v1.MapAdminEndpoints();
```

Add the using statement if not already present (it should be, since `TownCrier.Web.Endpoints` is already imported).

- [ ] **Step 4: Register types with JSON serializer context**

In `AppJsonSerializerContext.cs`, add these attributes:

```csharp
[JsonSerializable(typeof(GrantSubscriptionCommand))]
[JsonSerializable(typeof(GrantSubscriptionResult))]
```

Add the using statement:

```csharp
using TownCrier.Application.Admin;
```

- [ ] **Step 5: Run build**

Run: `dotnet build api`
Expected: BUILD SUCCEEDED.

- [ ] **Step 6: Run all tests**

Run: `dotnet test api/tests/town-crier.application.tests && dotnet test api/tests/town-crier.infrastructure.tests`
Expected: All tests PASS.

- [ ] **Step 7: Commit**

```bash
git add api/src/town-crier.web/Endpoints/AdminEndpoints.cs api/src/town-crier.web/Extensions/WebApplicationExtensions.cs api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs api/src/town-crier.web/AppJsonSerializerContext.cs
git commit -m "feat(web): wire up PUT /v1/admin/subscriptions endpoint"
```

---

### Task 9: Add `Admin:ApiKey` configuration

Add the configuration key to appsettings so the API knows what key to expect.

**Files:**
- Modify: `api/src/town-crier.web/appsettings.Development.json`

- [ ] **Step 1: Add development API key**

In `appsettings.Development.json`, add the `Admin` section:

```json
{
  "Logging": {
    "LogLevel": {
      "Default": "Information",
      "Microsoft.AspNetCore": "Warning"
    }
  },
  "Cors": {
    "AllowedOrigins": [
      "http://localhost:5173"
    ]
  },
  "Admin": {
    "ApiKey": "dev-admin-key-change-in-production"
  }
}
```

For production, the `Admin:ApiKey` value should be set via environment variable `Admin__ApiKey` or Azure Container Apps secrets — **not** committed to `appsettings.json`.

- [ ] **Step 2: Commit**

```bash
git add api/src/town-crier.web/appsettings.Development.json
git commit -m "chore(config): add Admin:ApiKey to development settings"
```

---

### Task 10: Final verification

Run the full test suite and verify the build.

**Files:** None (verification only)

- [ ] **Step 1: Run all tests**

Run: `dotnet test api`
Expected: All tests PASS across all test projects.

- [ ] **Step 2: Run format check**

Run: `dotnet format api --verify-no-changes`
Expected: No formatting issues.

- [ ] **Step 3: Verify build**

Run: `dotnet build api`
Expected: BUILD SUCCEEDED with no warnings.

- [ ] **Step 4: Final commit (if format fixes needed)**

If `dotnet format` found issues:

```bash
dotnet format api
git add -A
git commit -m "style: apply dotnet format"
```
