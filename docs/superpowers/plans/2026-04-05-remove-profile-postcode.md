# Remove Vestigial Postcode from UserProfile

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the unused `Postcode` field from `UserProfile` across API, web, and tests. Watch zones are the real mechanism for location tracking; the profile-level postcode is never written during onboarding and always shows "Not set."

**Architecture:** Strip `Postcode` from the domain model, Cosmos document mapping, all CQRS command/query/result records, the PATCH endpoint, web types, and the Settings page. The `Postcode` value object, geocoding service, `PlanningApplication.Postcode`, and iOS `WatchZone.postcode` are unaffected — they serve different purposes.

**Tech Stack:** .NET 10 (C#), React/TypeScript, TUnit, Vitest

---

### Task 1: API Domain — Remove Postcode from UserProfile

**Files:**
- Modify: `api/src/town-crier.domain/UserProfiles/UserProfile.cs`

- [ ] **Step 1: Remove `Postcode` from constructor, property, `Register`, `UpdatePreferences`, and `Reconstitute`**

In `UserProfile.cs`, make these changes:

**Constructor** — remove the `string? postcode` parameter and the `this.Postcode = postcode;` assignment:

```csharp
private UserProfile(
    string userId,
    string? email,
    NotificationPreferences notificationPreferences,
    SubscriptionTier tier,
    DateTimeOffset? subscriptionExpiry,
    string? originalTransactionId,
    DateTimeOffset? gracePeriodExpiry)
{
    this.UserId = userId;
    this.Email = email;
    this.NotificationPreferences = notificationPreferences;
    this.Tier = tier;
    this.SubscriptionExpiry = subscriptionExpiry;
    this.OriginalTransactionId = originalTransactionId;
    this.GracePeriodExpiry = gracePeriodExpiry;
}
```

**Remove the property:**
```csharp
// DELETE this line:
public string? Postcode { get; private set; }
```

**`Register`** — remove `postcode: null` from the constructor call:
```csharp
public static UserProfile Register(string userId, string? email = null)
{
    ArgumentException.ThrowIfNullOrWhiteSpace(userId);

    return new UserProfile(
        userId,
        email,
        notificationPreferences: NotificationPreferences.Default,
        tier: SubscriptionTier.Free,
        subscriptionExpiry: null,
        originalTransactionId: null,
        gracePeriodExpiry: null);
}
```

**`UpdatePreferences`** — remove the `string? postcode` parameter and the assignment:
```csharp
public void UpdatePreferences(NotificationPreferences notificationPreferences)
{
    this.NotificationPreferences = notificationPreferences;
}
```

**`Reconstitute`** — remove the `string? postcode` parameter and propagation:
```csharp
internal static UserProfile Reconstitute(
    string userId,
    string? email,
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

- [ ] **Step 2: Verify the project compiles (expect downstream failures — that's fine)**

Run: `dotnet build api/src/town-crier.domain/town-crier.domain.csproj`
Expected: SUCCESS (this project has no downstream references within itself)

- [ ] **Step 3: Commit**

```bash
git add api/src/town-crier.domain/UserProfiles/UserProfile.cs
git commit -m "refactor(domain): remove vestigial Postcode from UserProfile"
```

---

### Task 2: API Infrastructure — Remove Postcode from Cosmos Document Mapping

**Files:**
- Modify: `api/src/town-crier.infrastructure/UserProfiles/UserProfileDocument.cs`

- [ ] **Step 1: Remove `Postcode` from the document, `FromDomain`, and `ToDomain`**

**Remove the property:**
```csharp
// DELETE this line:
public string? Postcode { get; init; }
```

**In `FromDomain`** — remove the `Postcode = profile.Postcode,` line:
```csharp
public static UserProfileDocument FromDomain(UserProfile profile)
{
    ArgumentNullException.ThrowIfNull(profile);

    return new UserProfileDocument
    {
        Id = profile.UserId,
        UserId = profile.UserId,
        Email = profile.Email,
        PushEnabled = profile.NotificationPreferences.PushEnabled,
        DigestDay = profile.NotificationPreferences.DigestDay,
        EmailDigestEnabled = profile.NotificationPreferences.EmailDigestEnabled,
        EmailInstantEnabled = profile.NotificationPreferences.EmailInstantEnabled,
        ZonePreferences = new Dictionary<string, ZoneNotificationPreferences>(profile.AllZonePreferences),
        Tier = profile.Tier.ToString(),
        SubscriptionExpiry = profile.SubscriptionExpiry,
        OriginalTransactionId = profile.OriginalTransactionId,
        GracePeriodExpiry = profile.GracePeriodExpiry,
    };
}
```

**In `ToDomain`** — remove `this.Postcode` from the `Reconstitute` call (matches the updated signature from Task 1):
```csharp
public UserProfile ToDomain()
{
    var tier = Enum.Parse<SubscriptionTier>(this.Tier);
    var notificationPreferences = new NotificationPreferences(
        this.PushEnabled,
        this.DigestDay,
        this.EmailDigestEnabled,
        this.EmailInstantEnabled);

    return UserProfile.Reconstitute(
        this.UserId,
        this.Email,
        notificationPreferences,
        this.ZonePreferences,
        tier,
        this.SubscriptionExpiry,
        this.OriginalTransactionId,
        this.GracePeriodExpiry);
}
```

- [ ] **Step 2: Commit**

```bash
git add api/src/town-crier.infrastructure/UserProfiles/UserProfileDocument.cs
git commit -m "refactor(infra): remove Postcode from UserProfileDocument Cosmos mapping"
```

---

### Task 3: API Application — Remove Postcode from Commands, Results, and Handlers

**Files:**
- Modify: `api/src/town-crier.application/UserProfiles/UpdateUserProfileCommand.cs`
- Modify: `api/src/town-crier.application/UserProfiles/UpdateUserProfileResult.cs`
- Modify: `api/src/town-crier.application/UserProfiles/UpdateUserProfileCommandHandler.cs`
- Modify: `api/src/town-crier.application/UserProfiles/GetUserProfileResult.cs`
- Modify: `api/src/town-crier.application/UserProfiles/GetUserProfileQueryHandler.cs`
- Modify: `api/src/town-crier.application/UserProfiles/CreateUserProfileResult.cs`
- Modify: `api/src/town-crier.application/UserProfiles/CreateUserProfileCommandHandler.cs`
- Modify: `api/src/town-crier.application/UserProfiles/ExportUserDataResult.cs`
- Modify: `api/src/town-crier.application/UserProfiles/ExportUserDataQueryHandler.cs`

- [ ] **Step 1: Update `UpdateUserProfileCommand` — remove `Postcode` parameter**

```csharp
namespace TownCrier.Application.UserProfiles;

public sealed record UpdateUserProfileCommand(
    string UserId,
    bool PushEnabled);
```

- [ ] **Step 2: Update `UpdateUserProfileResult` — remove `Postcode` parameter**

```csharp
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.UserProfiles;

public sealed record UpdateUserProfileResult(
    string UserId,
    bool PushEnabled,
    SubscriptionTier Tier);
```

- [ ] **Step 3: Update `UpdateUserProfileCommandHandler` — remove postcode from `UpdatePreferences` call and result**

```csharp
public async Task<UpdateUserProfileResult> HandleAsync(UpdateUserProfileCommand command, CancellationToken ct)
{
    ArgumentNullException.ThrowIfNull(command);

    var profile = await this.repository.GetByUserIdAsync(command.UserId, ct).ConfigureAwait(false)
        ?? throw UserProfileNotFoundException.ForUser(command.UserId);

    profile.UpdatePreferences(new NotificationPreferences(command.PushEnabled));
    await this.repository.SaveAsync(profile, ct).ConfigureAwait(false);

    return new UpdateUserProfileResult(
        profile.UserId,
        profile.NotificationPreferences.PushEnabled,
        profile.Tier);
}
```

- [ ] **Step 4: Update `GetUserProfileResult` — remove `Postcode` parameter**

```csharp
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.UserProfiles;

public sealed record GetUserProfileResult(
    string UserId,
    bool PushEnabled,
    SubscriptionTier Tier);
```

- [ ] **Step 5: Update `GetUserProfileQueryHandler` — remove `profile.Postcode` from result construction**

```csharp
return new GetUserProfileResult(
    profile.UserId,
    profile.NotificationPreferences.PushEnabled,
    profile.Tier);
```

- [ ] **Step 6: Update `CreateUserProfileResult` — remove `Postcode` parameter**

```csharp
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.UserProfiles;

public sealed record CreateUserProfileResult(
    string UserId,
    bool PushEnabled,
    SubscriptionTier Tier);
```

- [ ] **Step 7: Update `CreateUserProfileCommandHandler` — remove `profile.Postcode` from both result constructions**

In the existing-profile branch (line 26-30):
```csharp
return new CreateUserProfileResult(
    existing.UserId,
    existing.NotificationPreferences.PushEnabled,
    existing.Tier);
```

In the new-profile branch (line 43-47):
```csharp
return new CreateUserProfileResult(
    profile.UserId,
    profile.NotificationPreferences.PushEnabled,
    profile.Tier);
```

- [ ] **Step 8: Update `ExportUserDataResult` — remove `Postcode` parameter**

```csharp
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.UserProfiles;

public sealed record ExportUserDataResult(
    string UserId,
    bool PushEnabled,
    SubscriptionTier Tier);
```

- [ ] **Step 9: Update `ExportUserDataQueryHandler` — remove `profile.Postcode` from result**

```csharp
return new ExportUserDataResult(
    profile.UserId,
    profile.NotificationPreferences.PushEnabled,
    profile.Tier);
```

- [ ] **Step 10: Commit**

```bash
git add api/src/town-crier.application/UserProfiles/
git commit -m "refactor(app): remove Postcode from all UserProfile commands, queries, and results"
```

---

### Task 4: API Endpoint — Remove Postcode from PATCH /v1/me

**Files:**
- Modify: `api/src/town-crier.web/Endpoints/UserProfileEndpoints.cs`

- [ ] **Step 1: Update the PATCH endpoint to stop reading/passing postcode**

Change line 40 from:
```csharp
var profileCommand = new UpdateUserProfileCommand(userId, command.Postcode, command.PushEnabled);
```
to:
```csharp
var profileCommand = new UpdateUserProfileCommand(userId, command.PushEnabled);
```

- [ ] **Step 2: Build the full API solution**

Run: `dotnet build api/`
Expected: SUCCESS (all postcode references in API source are now removed)

- [ ] **Step 3: Commit**

```bash
git add api/src/town-crier.web/Endpoints/UserProfileEndpoints.cs
git commit -m "refactor(web): remove Postcode from PATCH /v1/me endpoint"
```

---

### Task 5: API Tests — Update All Tests Referencing Profile Postcode

**Files:**
- Modify: `api/tests/town-crier.application.tests/UserProfiles/UpdateUserProfileCommandHandlerTests.cs`
- Modify: `api/tests/town-crier.application.tests/UserProfiles/GetUserProfileQueryHandlerTests.cs`
- Modify: `api/tests/town-crier.application.tests/UserProfiles/CreateUserProfileCommandHandlerTests.cs`
- Modify: `api/tests/town-crier.application.tests/UserProfiles/ExportUserDataQueryHandlerTests.cs`
- Modify: `api/tests/town-crier.application.tests/Notifications/UserProfileBuilder.cs`
- Modify: `api/tests/town-crier.infrastructure.tests/UserProfiles/UserProfileDocumentTests.cs`
- Modify: `api/tests/town-crier.infrastructure.tests/UserProfiles/UserProfileDocumentSerializationTests.cs`

- [ ] **Step 1: Update `UpdateUserProfileCommandHandlerTests`**

Rewrite the entire file:

```csharp
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.UserProfiles;

public sealed class UpdateUserProfileCommandHandlerTests
{
    [Test]
    public async Task Should_UpdatePushEnabled_When_ProfileExists()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-789");
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new UpdateUserProfileCommandHandler(repository);
        var command = new UpdateUserProfileCommand("auth0|user-789", true);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.PushEnabled).IsTrue();
    }

    [Test]
    public async Task Should_DisablePush_When_SetToFalse()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-789");
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new UpdateUserProfileCommandHandler(repository);
        var command = new UpdateUserProfileCommand("auth0|user-789", false);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.PushEnabled).IsFalse();
    }

    [Test]
    public async Task Should_PersistChanges_When_PreferencesUpdated()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-789");
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new UpdateUserProfileCommandHandler(repository);
        var command = new UpdateUserProfileCommand("auth0|user-789", false);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var saved = repository.GetByUserId("auth0|user-789");
        await Assert.That(saved!.NotificationPreferences.PushEnabled).IsFalse();
    }

    [Test]
    public async Task Should_ThrowUserProfileNotFoundException_When_ProfileDoesNotExist()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var handler = new UpdateUserProfileCommandHandler(repository);
        var command = new UpdateUserProfileCommand("auth0|nonexistent", true);

        // Act & Assert
        await Assert.ThrowsAsync<UserProfileNotFoundException>(
            () => handler.HandleAsync(command, CancellationToken.None));
    }

    [Test]
    public async Task Should_PreserveTier_When_PreferencesUpdated()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-789");
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new UpdateUserProfileCommandHandler(repository);
        var command = new UpdateUserProfileCommand("auth0|user-789", false);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert — tier should remain Free (not modifiable via preferences update)
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Free);
    }
}
```

- [ ] **Step 2: Update `GetUserProfileQueryHandlerTests` — remove `result.Postcode` assertion**

In `Should_ReturnProfile_When_ProfileExists`, delete this line:
```csharp
await Assert.That(result.Postcode).IsNull();
```

- [ ] **Step 3: Update `CreateUserProfileCommandHandlerTests` — remove `result.Postcode` assertion**

In `Should_CreateProfileWithFreeTier_When_NewUser`, delete this line:
```csharp
await Assert.That(result.Postcode).IsNull();
```

- [ ] **Step 4: Update `ExportUserDataQueryHandlerTests` — remove postcode from `UpdatePreferences` call and assertion**

In `Should_ReturnUserData_When_ProfileExists`:

Change:
```csharp
profile.UpdatePreferences("SW1A 1AA", new NotificationPreferences(PushEnabled: true));
```
to:
```csharp
profile.UpdatePreferences(new NotificationPreferences(PushEnabled: true));
```

Delete this assertion line:
```csharp
await Assert.That(result.Postcode).IsEqualTo("SW1A 1AA");
```

- [ ] **Step 5: Update `UserProfileBuilder` — remove postcode from `UpdatePreferences` call**

In the `Build()` method, change:
```csharp
profile.UpdatePreferences(
    postcode: null,
    new NotificationPreferences(
        this.pushEnabled,
        this.digestDay,
        this.emailDigestEnabled,
        this.emailInstantEnabled));
```
to:
```csharp
profile.UpdatePreferences(
    new NotificationPreferences(
        this.pushEnabled,
        this.digestDay,
        this.emailDigestEnabled,
        this.emailInstantEnabled));
```

- [ ] **Step 6: Update `UserProfileDocumentTests`**

In `Should_PreserveBasicFields_When_MappedFromDomain`:

Change:
```csharp
profile.UpdatePreferences("SW1A 1AA", new NotificationPreferences(true, DayOfWeek.Wednesday));
```
to:
```csharp
profile.UpdatePreferences(new NotificationPreferences(true, DayOfWeek.Wednesday));
```

Delete:
```csharp
await Assert.That(document.Postcode).IsEqualTo("SW1A 1AA");
```

In `Should_RoundTripToDomain_When_MappedBackAndForth`:

Change:
```csharp
original.UpdatePreferences("SW1A 1AA", new NotificationPreferences(false, DayOfWeek.Friday));
```
to:
```csharp
original.UpdatePreferences(new NotificationPreferences(false, DayOfWeek.Friday));
```

Delete:
```csharp
await Assert.That(roundTripped.Postcode).IsEqualTo(original.Postcode);
```

In `Should_HandleNullOptionalFields_When_MappedFromDomain`:

Delete:
```csharp
await Assert.That(document.Postcode).IsNull();
```

- [ ] **Step 7: Update `UserProfileDocumentSerializationTests`**

In `Should_RoundTripUserProfileDocument_When_Serialized`:

Change:
```csharp
profile.UpdatePreferences("SW1A 1AA", new NotificationPreferences(true, DayOfWeek.Wednesday));
```
to:
```csharp
profile.UpdatePreferences(new NotificationPreferences(true, DayOfWeek.Wednesday));
```

Delete:
```csharp
await Assert.That(deserialized.Postcode).IsEqualTo(original.Postcode);
```

- [ ] **Step 8: Run all API tests**

Run: `dotnet test api/`
Expected: All tests PASS

- [ ] **Step 9: Commit**

```bash
git add api/tests/
git commit -m "test(api): update all tests to remove profile postcode references"
```

---

### Task 6: Web — Remove Postcode from Types, Settings Page, and Tests

**Files:**
- Modify: `web/src/domain/types.ts`
- Modify: `web/src/features/Settings/SettingsPage.tsx`
- Modify: `web/src/features/Settings/__tests__/SettingsPage.test.tsx`
- Modify: `web/src/features/Settings/__tests__/fixtures/user-profile.fixtures.ts`

- [ ] **Step 1: Remove `postcode` from `UserProfile` type**

In `web/src/domain/types.ts`, change the `UserProfile` interface:
```typescript
export interface UserProfile {
  readonly userId: string;
  readonly pushEnabled: boolean;
  readonly tier: SubscriptionTier;
}
```

- [ ] **Step 2: Remove `postcode` from `UpdateProfileRequest` type**

In `web/src/domain/types.ts`, change:
```typescript
export interface UpdateProfileRequest {
  readonly pushEnabled: boolean;
}
```

- [ ] **Step 3: Remove the Postcode row from `SettingsPage.tsx`**

In `web/src/features/Settings/SettingsPage.tsx`, delete these 4 lines (59-62):
```tsx
<div className={styles.field}>
  <span className={styles.label}>Postcode</span>
  <span className={styles.value}>{profile?.postcode ?? 'Not set'}</span>
</div>
```

- [ ] **Step 4: Update test fixtures — remove `postcode` from `freeUserProfile`**

In `web/src/features/Settings/__tests__/fixtures/user-profile.fixtures.ts`:
```typescript
import type { UserProfile, SubscriptionTier } from '../../../../domain/types';

export function freeUserProfile(
  overrides?: Partial<UserProfile>,
): UserProfile {
  return {
    userId: 'auth0|abc123',
    pushEnabled: true,
    tier: 'Free' as SubscriptionTier,
    ...overrides,
  };
}

export function proUserProfile(
  overrides?: Partial<UserProfile>,
): UserProfile {
  return {
    ...freeUserProfile(),
    userId: 'auth0|pro456',
    tier: 'Pro' as SubscriptionTier,
    ...overrides,
  };
}
```

- [ ] **Step 5: Update `SettingsPage.test.tsx` — remove postcode test references**

In the `'renders profile info after loading'` test, change:
```typescript
it('renders profile info after loading', async () => {
  spy.fetchProfileResult = proUserProfile();

  renderSettingsPage(spy);

  expect(await screen.findByText('Pro')).toBeInTheDocument();
});
```

This removes the `postcode: 'SW1A 1AA'` override and the assertion for 'SW1A 1AA'.

- [ ] **Step 6: Run web type check and tests**

Run: `cd web && npx tsc --noEmit && npx vitest run`
Expected: All pass

- [ ] **Step 7: Commit**

```bash
git add web/src/domain/types.ts web/src/features/Settings/
git commit -m "refactor(web): remove postcode from UserProfile type and Settings page"
```
