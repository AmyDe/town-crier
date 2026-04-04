# Email Notifications Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add weekly email digests (all tiers) and instant notification emails (Personal/Pro) using Azure Communication Services.

**Architecture:** Extend the existing hexagonal notification system with a new `IEmailSender` port. The `GenerateWeeklyDigestsCommandHandler` gains email capability for all tiers alongside existing Pro-only push. The `DispatchNotificationCommandHandler` gains instant email for paid tiers. Azure Communication Services provides the email transport, provisioned via Pulumi with `hello@towncrierapp.uk` as the sender.

**Tech Stack:** .NET 10, Azure Communication Services Email SDK, Pulumi (C#), Cosmos DB, TUnit

**Spec:** `docs/specs/email-notifications.md`

---

### Task 1: Extend NotificationPreferences with email fields

**Files:**
- Modify: `api/src/town-crier.domain/UserProfiles/NotificationPreferences.cs`

- [ ] **Step 1: Add EmailDigestEnabled and EmailInstantEnabled to the record**

```csharp
namespace TownCrier.Domain.UserProfiles;

public sealed record NotificationPreferences(
    bool PushEnabled,
    DayOfWeek DigestDay = DayOfWeek.Monday,
    bool EmailDigestEnabled = true,
    bool EmailInstantEnabled = false)
{
    public static NotificationPreferences Default => new(PushEnabled: true);
}
```

The new parameters have defaults so all existing callers continue to compile. `EmailDigestEnabled` defaults to `true` (all tiers get digest email by default). `EmailInstantEnabled` defaults to `false` (opt-in for paid tiers).

- [ ] **Step 2: Verify build**

Run: `dotnet build api/api.sln`
Expected: BUILD SUCCEEDED — existing callers use positional/named args with existing params only, new params have defaults.

- [ ] **Step 3: Run existing tests to confirm no regressions**

Run: `dotnet test api/api.sln`
Expected: All tests pass.

- [ ] **Step 4: Commit**

```bash
git add api/src/town-crier.domain/UserProfiles/NotificationPreferences.cs
git commit -m "feat(api): add EmailDigestEnabled and EmailInstantEnabled to NotificationPreferences"
```

---

### Task 2: Update UserProfileDocument for new preferences fields

**Files:**
- Modify: `api/src/town-crier.infrastructure/UserProfiles/UserProfileDocument.cs`

The document flattens `NotificationPreferences` into individual properties. We need to add the two new fields with backwards-compatible defaults for existing documents in Cosmos that don't have these fields.

- [ ] **Step 1: Add EmailDigestEnabled and EmailInstantEnabled properties**

Add after the `DigestDay` property (line 17):

```csharp
    public bool EmailDigestEnabled { get; init; } = true;

    public bool EmailInstantEnabled { get; init; }
```

`EmailDigestEnabled` defaults to `true` so existing Cosmos documents without this field get the default. `EmailInstantEnabled` defaults to `false` (the `bool` default).

- [ ] **Step 2: Update FromDomain to include new fields**

In the `FromDomain` method, add after `DigestDay = ...`:

```csharp
            EmailDigestEnabled = profile.NotificationPreferences.EmailDigestEnabled,
            EmailInstantEnabled = profile.NotificationPreferences.EmailInstantEnabled,
```

- [ ] **Step 3: Update ToDomain to include new fields**

Change the `ToDomain` method's `NotificationPreferences` construction (line 52):

```csharp
        var notificationPreferences = new NotificationPreferences(
            this.PushEnabled,
            this.DigestDay,
            this.EmailDigestEnabled,
            this.EmailInstantEnabled);
```

- [ ] **Step 4: Verify build and tests**

Run: `dotnet build api/api.sln`
Run: `dotnet test api/api.sln`
Expected: All pass.

- [ ] **Step 5: Commit**

```bash
git add api/src/town-crier.infrastructure/UserProfiles/UserProfileDocument.cs
git commit -m "feat(api): serialize email preference fields in UserProfileDocument"
```

---

### Task 3: Update UserProfileBuilder for email preferences

**Files:**
- Modify: `api/tests/town-crier.application.tests/Notifications/UserProfileBuilder.cs`

- [ ] **Step 1: Add builder methods for email preferences**

Add fields after `digestDay` (line 11):

```csharp
    private bool emailDigestEnabled = true;
    private bool emailInstantEnabled;
```

Add methods after `WithDigestDay`:

```csharp
    public UserProfileBuilder WithEmailDigestEnabled(bool enabled)
    {
        this.emailDigestEnabled = enabled;
        return this;
    }

    public UserProfileBuilder WithEmailInstantEnabled(bool enabled)
    {
        this.emailInstantEnabled = enabled;
        return this;
    }
```

Update `Build()` to pass the new fields in the `NotificationPreferences` constructor:

```csharp
    public UserProfile Build()
    {
        var profile = UserProfile.Register(this.userId, this.email);
        profile.UpdatePreferences(
            postcode: null,
            new NotificationPreferences(
                this.pushEnabled,
                this.digestDay,
                this.emailDigestEnabled,
                this.emailInstantEnabled));

        if (this.tier != SubscriptionTier.Free)
        {
            profile.ActivateSubscription(this.tier, DateTimeOffset.UtcNow.AddYears(1));
        }

        return profile;
    }
```

- [ ] **Step 2: Verify build and tests**

Run: `dotnet test api/api.sln`
Expected: All pass — existing callers don't use the new builder methods yet.

- [ ] **Step 3: Commit**

```bash
git add api/tests/town-crier.application.tests/Notifications/UserProfileBuilder.cs
git commit -m "test(api): add email preference methods to UserProfileBuilder"
```

---

### Task 4: Create IEmailSender port and WatchZoneDigest DTO

**Files:**
- Create: `api/src/town-crier.application/Notifications/IEmailSender.cs`
- Create: `api/src/town-crier.application/Notifications/WatchZoneDigest.cs`

- [ ] **Step 1: Create WatchZoneDigest DTO**

```csharp
using TownCrier.Domain.Notifications;

namespace TownCrier.Application.Notifications;

public sealed record WatchZoneDigest(string WatchZoneName, IReadOnlyList<Notification> Notifications);
```

- [ ] **Step 2: Create IEmailSender interface**

```csharp
using TownCrier.Domain.Notifications;

namespace TownCrier.Application.Notifications;

public interface IEmailSender
{
    Task SendDigestAsync(string email, IReadOnlyList<WatchZoneDigest> digests, CancellationToken ct);

    Task SendNotificationAsync(string email, Notification notification, CancellationToken ct);
}
```

- [ ] **Step 3: Verify build**

Run: `dotnet build api/api.sln`
Expected: BUILD SUCCEEDED.

- [ ] **Step 4: Commit**

```bash
git add api/src/town-crier.application/Notifications/IEmailSender.cs api/src/town-crier.application/Notifications/WatchZoneDigest.cs
git commit -m "feat(api): add IEmailSender port and WatchZoneDigest DTO"
```

---

### Task 5: Create SpyEmailSender test double

**Files:**
- Create: `api/tests/town-crier.application.tests/Notifications/SpyEmailSender.cs`

- [ ] **Step 1: Create SpyEmailSender following the SpyPushNotificationSender pattern**

```csharp
using TownCrier.Application.Notifications;
using TownCrier.Domain.Notifications;

namespace TownCrier.Application.Tests.Notifications;

internal sealed class SpyEmailSender : IEmailSender
{
    private readonly List<(string Email, IReadOnlyList<WatchZoneDigest> Digests)> digestsSent = [];
    private readonly List<(string Email, Notification Notification)> notificationsSent = [];

    public IReadOnlyList<(string Email, IReadOnlyList<WatchZoneDigest> Digests)> DigestsSent => this.digestsSent;

    public IReadOnlyList<(string Email, Notification Notification)> NotificationsSent => this.notificationsSent;

    public Task SendDigestAsync(string email, IReadOnlyList<WatchZoneDigest> digests, CancellationToken ct)
    {
        this.digestsSent.Add((email, digests));
        return Task.CompletedTask;
    }

    public Task SendNotificationAsync(string email, Notification notification, CancellationToken ct)
    {
        this.notificationsSent.Add((email, notification));
        return Task.CompletedTask;
    }
}
```

- [ ] **Step 2: Verify build**

Run: `dotnet build api/api.sln`
Expected: BUILD SUCCEEDED.

- [ ] **Step 3: Commit**

```bash
git add api/tests/town-crier.application.tests/Notifications/SpyEmailSender.cs
git commit -m "test(api): add SpyEmailSender test double"
```

---

### Task 6: Add GetByUserSinceAsync (list) to INotificationRepository

The digest handler needs the actual notification records (not just the count) to build the email content.

**Files:**
- Modify: `api/src/town-crier.application/Notifications/INotificationRepository.cs`
- Modify: `api/tests/town-crier.application.tests/Notifications/FakeNotificationRepository.cs`
- Modify: `api/src/town-crier.infrastructure/Notifications/CosmosNotificationRepository.cs`

- [ ] **Step 1: Add method to INotificationRepository**

Add after `CountByUserSinceAsync` (line 11):

```csharp
    Task<IReadOnlyList<Notification>> GetByUserSinceAsync(string userId, DateTimeOffset since, CancellationToken ct);
```

- [ ] **Step 2: Implement in FakeNotificationRepository**

Add after `CountByUserSinceAsync` (line 38):

```csharp
    public Task<IReadOnlyList<Notification>> GetByUserSinceAsync(
        string userId, DateTimeOffset since, CancellationToken ct)
    {
        var notifications = this.store
            .Where(n => n.UserId == userId && n.CreatedAt >= since)
            .ToList();
        return Task.FromResult<IReadOnlyList<Notification>>(notifications);
    }
```

- [ ] **Step 3: Implement in CosmosNotificationRepository**

Add after the `CountByUserSinceAsync` method (after line 60):

```csharp
    public async Task<IReadOnlyList<Notification>> GetByUserSinceAsync(
        string userId, DateTimeOffset since, CancellationToken ct)
    {
        var documents = await this.client.QueryAsync(
            CosmosContainerNames.Notifications,
            "SELECT * FROM c WHERE c.userId = @userId AND c.createdAt >= @since ORDER BY c.createdAt DESC",
            [new QueryParameter("@userId", userId), new QueryParameter("@since", since)],
            userId,
            CosmosJsonSerializerContext.Default.NotificationDocument,
            ct).ConfigureAwait(false);

        return documents.ConvertAll(doc => doc.ToDomain());
    }
```

- [ ] **Step 4: Verify build and tests**

Run: `dotnet build api/api.sln`
Run: `dotnet test api/api.sln`
Expected: All pass.

- [ ] **Step 5: Commit**

```bash
git add api/src/town-crier.application/Notifications/INotificationRepository.cs api/tests/town-crier.application.tests/Notifications/FakeNotificationRepository.cs api/src/town-crier.infrastructure/Notifications/CosmosNotificationRepository.cs
git commit -m "feat(api): add GetByUserSinceAsync (list) to INotificationRepository"
```

---

### Task 7: Add GetAllByDigestDayAsync to IUserProfileRepository

The digest handler currently loads only Pro users via `GetAllByTierAsync(Pro)`. Email digests go to all tiers, so we need a query that returns all users whose `DigestDay` matches a given day.

**Files:**
- Modify: `api/src/town-crier.application/UserProfiles/IUserProfileRepository.cs`
- Modify: `api/tests/town-crier.application.tests/UserProfiles/FakeUserProfileRepository.cs`
- Modify: `api/src/town-crier.infrastructure/UserProfiles/InMemoryUserProfileRepository.cs`
- Modify: `api/src/town-crier.infrastructure/UserProfiles/CosmosUserProfileRepository.cs`

- [ ] **Step 1: Add method to IUserProfileRepository**

Add after `GetAllByTierAsync` (line 11):

```csharp
    Task<IReadOnlyList<UserProfile>> GetAllByDigestDayAsync(DayOfWeek digestDay, CancellationToken ct);
```

- [ ] **Step 2: Implement in FakeUserProfileRepository**

Add after `GetAllByTierAsync` (line 31):

```csharp
    public Task<IReadOnlyList<UserProfile>> GetAllByDigestDayAsync(DayOfWeek digestDay, CancellationToken ct)
    {
        var profiles = this.store.Values
            .Where(p => p.NotificationPreferences.DigestDay == digestDay)
            .ToList();
        return Task.FromResult<IReadOnlyList<UserProfile>>(profiles);
    }
```

- [ ] **Step 3: Implement in InMemoryUserProfileRepository**

Add after `GetAllByTierAsync` (line 22):

```csharp
    public Task<IReadOnlyList<UserProfile>> GetAllByDigestDayAsync(DayOfWeek digestDay, CancellationToken ct)
    {
        var profiles = this.store.Values
            .Where(p => p.NotificationPreferences.DigestDay == digestDay)
            .ToList();
        return Task.FromResult<IReadOnlyList<UserProfile>>(profiles);
    }
```

- [ ] **Step 4: Implement in CosmosUserProfileRepository**

Add after `GetAllByTierAsync` (line 53):

```csharp
    public async Task<IReadOnlyList<UserProfile>> GetAllByDigestDayAsync(DayOfWeek digestDay, CancellationToken ct)
    {
        var documents = await this.client.QueryAsync(
            CosmosContainerNames.Users,
            "SELECT * FROM c WHERE c.digestDay = @digestDay",
            [new QueryParameter("@digestDay", (int)digestDay)],
            partitionKey: null,
            CosmosJsonSerializerContext.Default.UserProfileDocument,
            ct).ConfigureAwait(false);

        return documents.ConvertAll(doc => doc.ToDomain());
    }
```

Note: `DayOfWeek` serializes as its integer value (0=Sunday through 6=Saturday) via `System.Text.Json` default behavior.

- [ ] **Step 5: Verify build and tests**

Run: `dotnet build api/api.sln`
Run: `dotnet test api/api.sln`
Expected: All pass.

- [ ] **Step 6: Commit**

```bash
git add api/src/town-crier.application/UserProfiles/IUserProfileRepository.cs api/tests/town-crier.application.tests/UserProfiles/FakeUserProfileRepository.cs api/src/town-crier.infrastructure/UserProfiles/InMemoryUserProfileRepository.cs api/src/town-crier.infrastructure/UserProfiles/CosmosUserProfileRepository.cs
git commit -m "feat(api): add GetAllByDigestDayAsync to IUserProfileRepository"
```

---

### Task 8: Refactor GenerateWeeklyDigestsCommandHandler for email digests (TDD)

The handler currently sends push digests to Pro users only. We refactor it to:
1. Load all users whose DigestDay matches today (not just Pro)
2. Load actual notification records (not just count)
3. Send push digest to Pro users with PushEnabled (existing behavior)
4. Send email digest to all users with EmailDigestEnabled + email address

**Files:**
- Modify: `api/src/town-crier.application/Notifications/GenerateWeeklyDigestsCommandHandler.cs`
- Modify: `api/tests/town-crier.application.tests/Notifications/GenerateWeeklyDigestsCommandHandlerTests.cs`

- [ ] **Step 1: Write failing test — free user with email gets digest email**

Add to `GenerateWeeklyDigestsCommandHandlerTests.cs`:

```csharp
    [Test]
    public async Task Should_SendDigestEmail_When_FreeUserHasEmailAndNotifications()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, _, _, emailSender, _) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithEmail("test@example.com")
            .WithTier(SubscriptionTier.Free)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        SeedNotificationsWithZone(notificationRepo, "user-1", "zone-1", count: 3,
            createdAt: MondayMarch2026.AddDays(-2));

        // Act
        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        // Assert
        await Assert.That(emailSender.DigestsSent).HasCount().EqualTo(1);
        await Assert.That(emailSender.DigestsSent[0].Email).IsEqualTo("test@example.com");
        await Assert.That(emailSender.DigestsSent[0].Digests).HasCount().EqualTo(1);
        await Assert.That(emailSender.DigestsSent[0].Digests[0].Notifications).HasCount().EqualTo(3);
    }
```

- [ ] **Step 2: Run test to verify it fails**

Run: `dotnet test api/tests/town-crier.application.tests --filter "Should_SendDigestEmail_When_FreeUserHasEmailAndNotifications"`
Expected: FAIL — `CreateHandler` signature doesn't include emailSender yet.

- [ ] **Step 3: Write failing test — user with EmailDigestEnabled=false gets no email**

```csharp
    [Test]
    public async Task Should_NotSendDigestEmail_When_EmailDigestDisabled()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, _, _, emailSender, _) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithEmail("test@example.com")
            .WithEmailDigestEnabled(false)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        SeedNotificationsWithZone(notificationRepo, "user-1", "zone-1", count: 3,
            createdAt: MondayMarch2026.AddDays(-2));

        // Act
        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        // Assert
        await Assert.That(emailSender.DigestsSent).HasCount().EqualTo(0);
    }
```

- [ ] **Step 4: Write failing test — user without email address gets no email**

```csharp
    [Test]
    public async Task Should_NotSendDigestEmail_When_NoEmailAddress()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, _, _, emailSender, _) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Free)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        SeedNotificationsWithZone(notificationRepo, "user-1", "zone-1", count: 3,
            createdAt: MondayMarch2026.AddDays(-2));

        // Act
        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        // Assert
        await Assert.That(emailSender.DigestsSent).HasCount().EqualTo(0);
    }
```

- [ ] **Step 5: Write failing test — Pro user gets both push and email**

```csharp
    [Test]
    public async Task Should_SendBothPushAndEmail_When_ProUserHasBothEnabled()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, emailSender, _) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithEmail("pro@example.com")
            .WithTier(SubscriptionTier.Pro)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var device = DeviceRegistration.Create("user-1", "device-1", DevicePlatform.Ios, MondayMarch2026);
        await deviceRepo.SaveAsync(device, CancellationToken.None);

        SeedNotificationsWithZone(notificationRepo, "user-1", "zone-1", count: 2,
            createdAt: MondayMarch2026.AddDays(-1));

        // Act
        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        // Assert
        await Assert.That(pushSender.DigestsSent).HasCount().EqualTo(1);
        await Assert.That(emailSender.DigestsSent).HasCount().EqualTo(1);
    }
```

- [ ] **Step 6: Write failing test — notifications grouped by watch zone**

```csharp
    [Test]
    public async Task Should_GroupNotificationsByWatchZone_When_SendingDigestEmail()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, _, _, emailSender, watchZoneRepo) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithEmail("test@example.com")
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var zone1 = new WatchZoneBuilder().WithId("zone-1").WithUserId("user-1").WithName("Home").Build();
        var zone2 = new WatchZoneBuilder().WithId("zone-2").WithUserId("user-1").WithName("Office").Build();
        await watchZoneRepo.SaveAsync(zone1, CancellationToken.None);
        await watchZoneRepo.SaveAsync(zone2, CancellationToken.None);

        SeedNotificationsWithZone(notificationRepo, "user-1", "zone-1", count: 2,
            createdAt: MondayMarch2026.AddDays(-1));
        SeedNotificationsWithZone(notificationRepo, "user-1", "zone-2", count: 3,
            createdAt: MondayMarch2026.AddDays(-2));

        // Act
        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        // Assert
        await Assert.That(emailSender.DigestsSent).HasCount().EqualTo(1);
        var digests = emailSender.DigestsSent[0].Digests;
        await Assert.That(digests).HasCount().EqualTo(2);
        await Assert.That(digests.Any(d => d.WatchZoneName == "Home" && d.Notifications.Count == 2)).IsTrue();
        await Assert.That(digests.Any(d => d.WatchZoneName == "Office" && d.Notifications.Count == 3)).IsTrue();
    }
```

- [ ] **Step 7: Update CreateHandler and add helpers**

Update the `CreateHandler` method and add helper methods:

```csharp
    private static (GenerateWeeklyDigestsCommandHandler Handler,
        FakeNotificationRepository NotificationRepo,
        FakeUserProfileRepository UserProfileRepo,
        SpyPushNotificationSender PushSender,
        FakeDeviceRegistrationRepository DeviceRepo,
        SpyEmailSender EmailSender,
        FakeWatchZoneRepository WatchZoneRepo) CreateHandler(FakeTimeProvider? timeProvider = null)
    {
        var notificationRepo = new FakeNotificationRepository();
        var userProfileRepo = new FakeUserProfileRepository();
        var deviceRepo = new FakeDeviceRegistrationRepository();
        var pushSender = new SpyPushNotificationSender();
        var emailSender = new SpyEmailSender();
        var watchZoneRepo = new FakeWatchZoneRepository();
        var tp = timeProvider ?? new FakeTimeProvider(MondayMarch2026);

        var handler = new GenerateWeeklyDigestsCommandHandler(
            userProfileRepo, notificationRepo, deviceRepo, pushSender, emailSender, watchZoneRepo, tp);

        return (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, emailSender, watchZoneRepo);
    }
```

Update `SeedProUserWithDevice` to set email:

```csharp
    private static async Task SeedProUserWithDevice(
        FakeUserProfileRepository userProfileRepo,
        FakeDeviceRegistrationRepository deviceRepo,
        string userId,
        DayOfWeek digestDay = DayOfWeek.Monday)
    {
        var profile = new UserProfileBuilder()
            .WithUserId(userId)
            .WithEmail($"{userId}@example.com")
            .WithTier(SubscriptionTier.Pro)
            .WithDigestDay(digestDay)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var device = DeviceRegistration.Create(userId, $"device-{userId}", DevicePlatform.Ios, MondayMarch2026);
        await deviceRepo.SaveAsync(device, CancellationToken.None);
    }
```

Add zone-aware notification seeding:

```csharp
    private static void SeedNotificationsWithZone(
        FakeNotificationRepository notificationRepo,
        string userId,
        string watchZoneId,
        int count,
        DateTimeOffset createdAt)
    {
        for (var i = 0; i < count; i++)
        {
            var notification = Notification.Create(
                userId: userId,
                applicationName: $"app-{watchZoneId}-{i:D3}",
                watchZoneId: watchZoneId,
                applicationAddress: $"{i} Test Street",
                applicationDescription: "Test application",
                applicationType: "Full",
                authorityId: 1,
                now: createdAt);
            notificationRepo.Seed(notification);
        }
    }
```

Update existing `SeedNotifications` to delegate to `SeedNotificationsWithZone` so existing test calls don't need to change:

```csharp
    private static void SeedNotifications(
        FakeNotificationRepository notificationRepo,
        string userId,
        int count,
        DateTimeOffset createdAt)
    {
        SeedNotificationsWithZone(notificationRepo, userId, "zone-1", count, createdAt);
    }
```

Also update existing test destructuring from 5-tuple to 7-tuple — for existing tests that don't need the new fakes, use discards:

```csharp
var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, _, _) = CreateHandler();
```

Add the necessary `using` directives at the top of the test file:

```csharp
using TownCrier.Application.Tests.Polling;
```

This imports `FakeWatchZoneRepository` and `WatchZoneBuilder`.

- [ ] **Step 8: Implement the handler changes**

Replace the full `GenerateWeeklyDigestsCommandHandler`:

```csharp
using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.UserProfiles;
using TownCrier.Application.WatchZones;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Notifications;

public sealed class GenerateWeeklyDigestsCommandHandler
{
    private readonly IUserProfileRepository userProfileRepository;
    private readonly INotificationRepository notificationRepository;
    private readonly IDeviceRegistrationRepository deviceRegistrationRepository;
    private readonly IPushNotificationSender pushNotificationSender;
    private readonly IEmailSender emailSender;
    private readonly IWatchZoneRepository watchZoneRepository;
    private readonly TimeProvider timeProvider;

    public GenerateWeeklyDigestsCommandHandler(
        IUserProfileRepository userProfileRepository,
        INotificationRepository notificationRepository,
        IDeviceRegistrationRepository deviceRegistrationRepository,
        IPushNotificationSender pushNotificationSender,
        IEmailSender emailSender,
        IWatchZoneRepository watchZoneRepository,
        TimeProvider timeProvider)
    {
        this.userProfileRepository = userProfileRepository;
        this.notificationRepository = notificationRepository;
        this.deviceRegistrationRepository = deviceRegistrationRepository;
        this.pushNotificationSender = pushNotificationSender;
        this.emailSender = emailSender;
        this.watchZoneRepository = watchZoneRepository;
        this.timeProvider = timeProvider;
    }

    public async Task HandleAsync(GenerateWeeklyDigestsCommand command, CancellationToken ct)
    {
        var now = this.timeProvider.GetUtcNow();
        var today = now.DayOfWeek;
        var since = now.AddDays(-7);

        var users = await this.userProfileRepository.GetAllByDigestDayAsync(today, ct)
            .ConfigureAwait(false);

        foreach (var profile in users)
        {
            var wantsPush = profile.Tier == SubscriptionTier.Pro
                && profile.NotificationPreferences.PushEnabled;
            var wantsEmail = profile.NotificationPreferences.EmailDigestEnabled
                && !string.IsNullOrEmpty(profile.Email);

            if (!wantsPush && !wantsEmail)
            {
                continue;
            }

            var notifications = await this.notificationRepository.GetByUserSinceAsync(
                profile.UserId, since, ct).ConfigureAwait(false);

            if (notifications.Count == 0)
            {
                continue;
            }

            if (wantsPush)
            {
                var devices = await this.deviceRegistrationRepository.GetByUserIdAsync(profile.UserId, ct)
                    .ConfigureAwait(false);

                if (devices.Count > 0)
                {
                    await this.pushNotificationSender.SendDigestAsync(notifications.Count, devices, ct)
                        .ConfigureAwait(false);
                }
            }

            if (wantsEmail)
            {
                var zones = await this.watchZoneRepository.GetByUserIdAsync(profile.UserId, ct)
                    .ConfigureAwait(false);

                var zoneLookup = zones.ToDictionary(z => z.Id, z => z.Name);

                var digests = notifications
                    .GroupBy(n => n.WatchZoneId)
                    .Select(g => new WatchZoneDigest(
                        zoneLookup.GetValueOrDefault(g.Key, "Unknown Zone"),
                        g.ToList()))
                    .ToList();

                await this.emailSender.SendDigestAsync(profile.Email!, digests, ct)
                    .ConfigureAwait(false);
            }
        }
    }
}
```

- [ ] **Step 9: Run all tests**

Run: `dotnet test api/tests/town-crier.application.tests --filter "GenerateWeeklyDigests"`
Expected: All existing tests pass (with updated destructuring). All new tests pass.

- [ ] **Step 10: Commit**

```bash
git add api/src/town-crier.application/Notifications/GenerateWeeklyDigestsCommandHandler.cs api/tests/town-crier.application.tests/Notifications/GenerateWeeklyDigestsCommandHandlerTests.cs
git commit -m "feat(api): add email digest sending to GenerateWeeklyDigestsCommandHandler"
```

---

### Task 9: Extend DispatchNotificationCommandHandler for instant emails (TDD)

The handler currently sends push notifications. For Personal/Pro users with `EmailInstantEnabled`, it should also send an email after the push logic.

**Files:**
- Modify: `api/src/town-crier.application/Notifications/DispatchNotificationCommandHandler.cs`
- Modify: `api/tests/town-crier.application.tests/Notifications/DispatchNotificationCommandHandlerTests.cs`

- [ ] **Step 1: Write failing test — Personal user with EmailInstantEnabled gets email**

Add to `DispatchNotificationCommandHandlerTests.cs`:

```csharp
    [Test]
    public async Task Should_SendInstantEmail_When_PersonalUserHasEmailInstantEnabled()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, _, deviceRepo, emailSender) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithEmail("user@example.com")
            .WithTier(SubscriptionTier.Personal)
            .WithEmailInstantEnabled(true)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var device = DeviceRegistration.Create("user-1", "device-token-1", DevicePlatform.Ios, March2026);
        await deviceRepo.SaveAsync(device, CancellationToken.None);

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert
        await Assert.That(emailSender.NotificationsSent).HasCount().EqualTo(1);
        await Assert.That(emailSender.NotificationsSent[0].Email).IsEqualTo("user@example.com");
    }
```

- [ ] **Step 2: Write failing test — Free user does not get instant email**

```csharp
    [Test]
    public async Task Should_NotSendInstantEmail_When_FreeUserHasEmailInstantEnabled()
    {
        // Arrange
        var (handler, _, userProfileRepo, _, deviceRepo, emailSender) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithEmail("user@example.com")
            .WithEmailInstantEnabled(true)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var device = DeviceRegistration.Create("user-1", "device-token-1", DevicePlatform.Ios, March2026);
        await deviceRepo.SaveAsync(device, CancellationToken.None);

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert
        await Assert.That(emailSender.NotificationsSent).HasCount().EqualTo(0);
    }
```

- [ ] **Step 3: Write failing test — no email sent when EmailInstantEnabled is false**

```csharp
    [Test]
    public async Task Should_NotSendInstantEmail_When_EmailInstantDisabled()
    {
        // Arrange
        var (handler, _, userProfileRepo, _, deviceRepo, emailSender) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithEmail("user@example.com")
            .WithTier(SubscriptionTier.Pro)
            .WithEmailInstantEnabled(false)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var device = DeviceRegistration.Create("user-1", "device-token-1", DevicePlatform.Ios, March2026);
        await deviceRepo.SaveAsync(device, CancellationToken.None);

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert
        await Assert.That(emailSender.NotificationsSent).HasCount().EqualTo(0);
    }
```

- [ ] **Step 4: Update CreateHandler to include SpyEmailSender**

```csharp
    private static (DispatchNotificationCommandHandler Handler,
        FakeNotificationRepository NotificationRepo,
        FakeUserProfileRepository UserProfileRepo,
        SpyPushNotificationSender PushSender,
        FakeDeviceRegistrationRepository DeviceRepo,
        SpyEmailSender EmailSender) CreateHandler(FakeTimeProvider? timeProvider = null)
    {
        var notificationRepo = new FakeNotificationRepository();
        var userProfileRepo = new FakeUserProfileRepository();
        var deviceRepo = new FakeDeviceRegistrationRepository();
        var pushSender = new SpyPushNotificationSender();
        var emailSender = new SpyEmailSender();
        var tp = timeProvider ?? new FakeTimeProvider(March2026);

        var handler = new DispatchNotificationCommandHandler(
            notificationRepo, userProfileRepo, deviceRepo, pushSender, emailSender, tp);

        return (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, emailSender);
    }
```

Update existing test destructuring from 5-tuple to 6-tuple. For tests that don't need emailSender, use discard:

```csharp
var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, _) = CreateHandler();
```

- [ ] **Step 5: Implement handler changes**

Add `IEmailSender` as a constructor dependency:

```csharp
    private readonly IEmailSender emailSender;
```

Add to constructor parameters and assignment:

```csharp
    public DispatchNotificationCommandHandler(
        INotificationRepository notificationRepository,
        IUserProfileRepository userProfileRepository,
        IDeviceRegistrationRepository deviceRegistrationRepository,
        IPushNotificationSender pushNotificationSender,
        IEmailSender emailSender,
        TimeProvider timeProvider)
    {
        this.notificationRepository = notificationRepository;
        this.userProfileRepository = userProfileRepository;
        this.deviceRegistrationRepository = deviceRegistrationRepository;
        this.pushNotificationSender = pushNotificationSender;
        this.emailSender = emailSender;
        this.timeProvider = timeProvider;
    }
```

Add email sending after the push block and before `SaveAsync`, at the end of `HandleAsync` (before the final `SaveAsync` call on line 113):

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

- [ ] **Step 6: Run all dispatch tests**

Run: `dotnet test api/tests/town-crier.application.tests --filter "DispatchNotification"`
Expected: All existing and new tests pass.

- [ ] **Step 7: Commit**

```bash
git add api/src/town-crier.application/Notifications/DispatchNotificationCommandHandler.cs api/tests/town-crier.application.tests/Notifications/DispatchNotificationCommandHandlerTests.cs
git commit -m "feat(api): add instant email sending to DispatchNotificationCommandHandler"
```

---

### Task 10: Create NoOpEmailSender and AcsEmailSender

**Files:**
- Create: `api/src/town-crier.infrastructure/Notifications/NoOpEmailSender.cs`
- Create: `api/src/town-crier.infrastructure/Notifications/AcsEmailSender.cs`

- [ ] **Step 1: Create NoOpEmailSender**

```csharp
using TownCrier.Application.Notifications;
using TownCrier.Domain.Notifications;

namespace TownCrier.Infrastructure.Notifications;

public sealed class NoOpEmailSender : IEmailSender
{
    public Task SendDigestAsync(string email, IReadOnlyList<WatchZoneDigest> digests, CancellationToken ct)
    {
        return Task.CompletedTask;
    }

    public Task SendNotificationAsync(string email, Notification notification, CancellationToken ct)
    {
        return Task.CompletedTask;
    }
}
```

- [ ] **Step 2: Create AcsEmailSender**

```csharp
using Azure;
using Azure.Communication.Email;
using TownCrier.Application.Notifications;
using TownCrier.Domain.Notifications;

namespace TownCrier.Infrastructure.Notifications;

public sealed class AcsEmailSender : IEmailSender
{
    private const string SenderAddress = "hello@towncrierapp.uk";
    private readonly EmailClient emailClient;

    public AcsEmailSender(string connectionString)
    {
        this.emailClient = new EmailClient(connectionString);
    }

    public async Task SendDigestAsync(string email, IReadOnlyList<WatchZoneDigest> digests, CancellationToken ct)
    {
        var totalCount = digests.Sum(d => d.Notifications.Count);
        var htmlBody = BuildDigestHtml(digests, totalCount);

        var emailMessage = new EmailMessage(
            senderAddress: SenderAddress,
            content: new EmailContent($"Your weekly planning digest — {totalCount} new applications")
            {
                Html = htmlBody,
            },
            recipients: new EmailRecipients([new EmailAddress(email)]));

        await this.emailClient.SendAsync(WaitUntil.Started, emailMessage, ct).ConfigureAwait(false);
    }

    public async Task SendNotificationAsync(string email, Notification notification, CancellationToken ct)
    {
        var htmlBody = BuildNotificationHtml(notification);

        var emailMessage = new EmailMessage(
            senderAddress: SenderAddress,
            content: new EmailContent($"New planning application — {notification.ApplicationAddress}")
            {
                Html = htmlBody,
            },
            recipients: new EmailRecipients([new EmailAddress(email)]));

        await this.emailClient.SendAsync(WaitUntil.Started, emailMessage, ct).ConfigureAwait(false);
    }

    private static string BuildDigestHtml(IReadOnlyList<WatchZoneDigest> digests, int totalCount)
    {
        var zoneBlocks = string.Join("", digests.Select(d =>
        {
            var cards = string.Join("", d.Notifications.Select(n => $"""
                <tr><td style="padding:0 0 8px 0;">
                  <table width="100%" cellpadding="0" cellspacing="0" style="background:#f8f9fa;border-radius:6px;">
                    <tr><td style="padding:12px;">
                      <div style="font-weight:600;color:#1a1a2e;">{HtmlEncode(n.ApplicationAddress)}</div>
                      <div style="color:#4a6cf7;font-size:13px;">{HtmlEncode(n.ApplicationType)}</div>
                      <div style="color:#666;font-size:13px;margin-top:4px;">{HtmlEncode(Truncate(n.ApplicationDescription, 120))}</div>
                    </td></tr>
                  </table>
                </td></tr>
                """));

            return $"""
                <tr><td style="padding:16px 0 8px 0;font-size:14px;color:#666;text-transform:uppercase;letter-spacing:0.5px;">
                  📍 {HtmlEncode(d.WatchZoneName)}
                </td></tr>
                {cards}
                """;
        }));

        return $"""
            <!DOCTYPE html>
            <html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"></head>
            <body style="margin:0;padding:0;background:#f0f0f0;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;">
            <table width="100%" cellpadding="0" cellspacing="0"><tr><td align="center" style="padding:24px;">
            <table width="600" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:8px;overflow:hidden;">
              <tr><td style="background:#1a1a2e;padding:24px;text-align:center;">
                <div style="font-size:20px;font-weight:700;color:#ffffff;">Town Crier</div>
                <div style="color:#888;font-size:13px;margin-top:4px;">Weekly Planning Digest</div>
              </td></tr>
              <tr><td style="padding:24px;">
                <table width="100%" cellpadding="0" cellspacing="0">
                  {zoneBlocks}
                </table>
                <table width="100%" cellpadding="0" cellspacing="0" style="margin-top:24px;">
                  <tr><td align="center">
                    <a href="https://towncrierapp.uk" style="display:inline-block;background:#4a6cf7;color:#ffffff;padding:12px 32px;border-radius:6px;text-decoration:none;font-weight:600;">View All in App</a>
                  </td></tr>
                </table>
              </td></tr>
              <tr><td style="padding:16px 24px;text-align:center;color:#999;font-size:12px;border-top:1px solid #eee;">
                {totalCount} application{(totalCount != 1 ? "s" : "")} this week · <a href="https://towncrierapp.uk/settings" style="color:#999;">Unsubscribe</a>
              </td></tr>
            </table>
            </td></tr></table>
            </body></html>
            """;
    }

    private static string BuildNotificationHtml(Notification notification)
    {
        return $"""
            <!DOCTYPE html>
            <html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"></head>
            <body style="margin:0;padding:0;background:#f0f0f0;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;">
            <table width="100%" cellpadding="0" cellspacing="0"><tr><td align="center" style="padding:24px;">
            <table width="600" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:8px;overflow:hidden;">
              <tr><td style="background:#1a1a2e;padding:24px;text-align:center;">
                <div style="font-size:20px;font-weight:700;color:#ffffff;">Town Crier</div>
                <div style="color:#888;font-size:13px;margin-top:4px;">New Planning Application</div>
              </td></tr>
              <tr><td style="padding:24px;">
                <div style="font-size:18px;font-weight:600;color:#1a1a2e;">{HtmlEncode(notification.ApplicationAddress)}</div>
                <div style="color:#4a6cf7;font-size:14px;margin-top:4px;">{HtmlEncode(notification.ApplicationType)}</div>
                <div style="color:#666;font-size:14px;margin-top:12px;">{HtmlEncode(notification.ApplicationDescription)}</div>
                <table width="100%" cellpadding="0" cellspacing="0" style="margin-top:24px;">
                  <tr><td align="center">
                    <a href="https://towncrierapp.uk" style="display:inline-block;background:#4a6cf7;color:#ffffff;padding:12px 32px;border-radius:6px;text-decoration:none;font-weight:600;">View in App</a>
                  </td></tr>
                </table>
              </td></tr>
              <tr><td style="padding:16px 24px;text-align:center;color:#999;font-size:12px;border-top:1px solid #eee;">
                <a href="https://towncrierapp.uk/settings" style="color:#999;">Manage notifications</a>
              </td></tr>
            </table>
            </td></tr></table>
            </body></html>
            """;
    }

    private static string HtmlEncode(string text)
    {
        return System.Net.WebUtility.HtmlEncode(text);
    }

    private static string Truncate(string text, int maxLength)
    {
        return text.Length <= maxLength ? text : string.Concat(text.AsSpan(0, maxLength - 1), "…");
    }
}
```

- [ ] **Step 3: Add Azure.Communication.Email NuGet package to infrastructure project**

Run: `dotnet add api/src/town-crier.infrastructure/town-crier.infrastructure.csproj package Azure.Communication.Email`

- [ ] **Step 4: Verify build**

Run: `dotnet build api/api.sln`
Expected: BUILD SUCCEEDED.

If Native AOT trimming fails with the ACS SDK, fall back to a direct HTTP REST implementation using `HttpClient`. The ACS Email REST API endpoint is `POST https://{endpoint}/emails:send?api-version=2023-03-31`. This is a contingency — try the SDK first.

- [ ] **Step 5: Commit**

```bash
git add api/src/town-crier.infrastructure/Notifications/NoOpEmailSender.cs api/src/town-crier.infrastructure/Notifications/AcsEmailSender.cs api/src/town-crier.infrastructure/town-crier.infrastructure.csproj
git commit -m "feat(api): add NoOpEmailSender and AcsEmailSender implementations"
```

---

### Task 11: Wire up DI registration

**Files:**
- Modify: `api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs`

- [ ] **Step 1: Register IEmailSender in AddInfrastructureServices**

Add the using directive at the top:

```csharp
using Azure.Communication.Email;
```

After the `INotificationRepository` registration (line 46), add:

```csharp
        var acsConnectionString = configuration["AzureCommunicationServices:ConnectionString"];
        if (!string.IsNullOrEmpty(acsConnectionString))
        {
            services.AddSingleton<IEmailSender>(new AcsEmailSender(acsConnectionString));
        }
        else
        {
            services.AddSingleton<IEmailSender, NoOpEmailSender>();
        }
```

- [ ] **Step 2: Register the handlers that now need IEmailSender**

The handlers are manually constructed (not registered in DI) in the current codebase, so no additional registration is needed for them. The `IEmailSender` registration is sufficient — whoever constructs the handlers will resolve it from the container.

- [ ] **Step 3: Verify build**

Run: `dotnet build api/api.sln`
Expected: BUILD SUCCEEDED.

- [ ] **Step 4: Commit**

```bash
git add api/src/town-crier.web/Extensions/ServiceCollectionExtensions.cs
git commit -m "feat(api): register IEmailSender in DI with ACS fallback to NoOp"
```

---

### Task 12: Provision ACS infrastructure via Pulumi

**Files:**
- Modify: `infra/SharedStack.cs`
- Modify: `infra/EnvironmentStack.cs`

- [ ] **Step 1: Add Azure Communication Services to SharedStack**

Add using directive:

```csharp
using Pulumi.AzureNative.Communication;
using Pulumi.AzureNative.Communication.Inputs;
```

Add after the Cosmos DB role assignment block (after line 168):

```csharp
        // Azure Communication Services (Email)
        var communicationService = new CommunicationService("acs-town-crier-shared", new CommunicationServiceArgs
        {
            CommunicationServiceName = "acs-town-crier-shared",
            ResourceGroupName = resourceGroup.Name,
            DataLocation = "Europe",
            Tags = tags,
        });

        var emailService = new EmailService("email-town-crier-shared", new EmailServiceArgs
        {
            EmailServiceName = "email-town-crier-shared",
            ResourceGroupName = resourceGroup.Name,
            DataLocation = "Europe",
            Tags = tags,
        });

        var emailDomain = new Domain("domain-towncrierapp-uk", new DomainArgs
        {
            DomainName = "towncrierapp.uk",
            EmailServiceName = emailService.Name,
            ResourceGroupName = resourceGroup.Name,
            DomainManagement = DomainManagement.CustomerManaged,
            Tags = tags,
        });
```

Add to the outputs dictionary:

```csharp
            ["acsConnectionString"] = communicationService.HostName.Apply(h => $"endpoint=https://{h}/"),
```

Note: The actual connection string with keys may need to be retrieved separately. Check if the Pulumi resource exposes a primary connection string directly. If not, use `az communication list-key` post-deploy to get the full connection string and store it as a Pulumi secret.

- [ ] **Step 2: Pass ACS connection string to Container App in EnvironmentStack**

Add to the shared stack output reads (after line 48):

```csharp
        var acsConnectionString = shared.GetOutput("acsConnectionString").Apply(o => o?.ToString() ?? "");
```

Add a secret for the ACS connection string in the Container App Configuration.Secrets array:

```csharp
new SecretArgs { Name = "acs-connection-string", Value = acsConnectionString },
```

Add to the Container App's Env array:

```csharp
new EnvironmentVarArgs { Name = "AzureCommunicationServices__ConnectionString", SecretRef = "acs-connection-string" },
```

- [ ] **Step 3: Verify Pulumi build**

Run: `cd infra && dotnet build`
Expected: BUILD SUCCEEDED.

- [ ] **Step 4: Commit**

```bash
git add infra/SharedStack.cs infra/EnvironmentStack.cs
git commit -m "infra: provision Azure Communication Services for email"
```

---

### Task 13: Add ADR and update memo status

**Files:**
- Create: `docs/adr/0020-email-notifications-via-acs.md`
- Modify: `docs/memo/0002-weekly-email-digest-delivery.md`

- [ ] **Step 1: Create ADR**

```markdown
# 0020. Email Notifications via Azure Communication Services

Date: 2026-04-04

## Status

Accepted

## Context

Town Crier needs email as a notification channel — weekly digest emails for all subscription tiers and instant notification emails for Personal/Pro users. The memo (0002) analysed cost and vendor options.

## Decision

Use Azure Communication Services (ACS) Email with the `towncrierapp.uk` custom domain. ACS is provisioned via Pulumi alongside existing Azure resources — no new vendor. The sender address is `hello@towncrierapp.uk`.

The implementation extends the existing hexagonal notification architecture:
- New `IEmailSender` port with `AcsEmailSender` adapter
- `GenerateWeeklyDigestsCommandHandler` sends email digests to all tiers (grouped by watch zone, card-per-application layout)
- `DispatchNotificationCommandHandler` sends instant emails to Personal/Pro users with `EmailInstantEnabled`
- Email preferences (`EmailDigestEnabled`, `EmailInstantEnabled`) added to `NotificationPreferences`
- Email address read from user profile in Cosmos (set during onboarding)

Weekly digests are triggered by a daily Container Apps Job (shared with the future cron infrastructure). Instant emails fire inline with the existing change feed notification path.

## Consequences

- Near-zero marginal cost (~$0.75/month at 4K emails)
- Email becomes the primary engagement channel for free-tier users
- Custom domain requires DNS verification records in Cloudflare (SPF, DKIM, DMARC)
- ACS SDK dependency added to infrastructure layer — must verify Native AOT compatibility
- If ACS SDK has AOT issues, fallback to direct REST API
```

- [ ] **Step 2: Update memo status**

Change line 6 of `docs/memo/0002-weekly-email-digest-delivery.md` from `Open` to:

```
Superseded by ADR [0020](../adr/0020-email-notifications-via-acs.md)
```

- [ ] **Step 3: Commit**

```bash
git add docs/adr/0020-email-notifications-via-acs.md docs/memo/0002-weekly-email-digest-delivery.md
git commit -m "docs: add ADR 0020 for email notifications, supersede memo 0002"
```

---

### Follow-Up: Daily Digest Container Apps Job

This plan implements all handler logic, ports, adapters, and infrastructure resources. The one missing piece is the **trigger** — a daily Container Apps Job that invokes `GenerateWeeklyDigestsCommand`. This requires:

1. A new console app (e.g. `town-crier.digest-worker`) or a `--command=digest` flag on the existing `town-crier.worker`
2. A second Container Apps Job in `EnvironmentStack.cs` with `CronExpression = "0 8 * * *"` (daily at 08:00 UTC)
3. CI/CD pipeline changes to build and deploy the digest worker image

This should be created as a separate bead and can reuse the pattern from the polling job (`job-town-crier-poll-{env}`). The handler is ready and fully tested — it just needs a trigger.

### Post-Implementation: DNS Setup (Manual)

After deploying the Pulumi changes, configure DNS records in Cloudflare for domain verification. ACS provides the required records during domain setup:

1. SPF record: `TXT` on `towncrierapp.uk` — value provided by ACS
2. DKIM records: `CNAME` entries — provided by ACS
3. DMARC record: `TXT` on `_dmarc.towncrierapp.uk` — e.g. `v=DMARC1; p=quarantine; rua=mailto:dmarc@towncrierapp.uk`

Verify domain in Azure portal or via `az communication email domain show`. This is a one-time manual step post-deploy.
