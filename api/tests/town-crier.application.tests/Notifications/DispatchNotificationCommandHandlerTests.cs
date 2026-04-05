using TownCrier.Application.Notifications;
using TownCrier.Application.Tests.Polling;
using TownCrier.Application.Tests.UserProfiles;
using TownCrier.Domain.DeviceRegistrations;
using TownCrier.Domain.UserProfiles;
using FakeDeviceRegistrationRepository = TownCrier.Application.Tests.DeviceRegistrations.FakeDeviceRegistrationRepository;

namespace TownCrier.Application.Tests.Notifications;

public sealed class DispatchNotificationCommandHandlerTests
{
    private static readonly DateTimeOffset March2026 = new(2026, 3, 15, 10, 0, 0, TimeSpan.Zero);

    [Test]
    public async Task Should_CreateNotificationRecord_When_ApplicationMatchesWatchZone()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, _, deviceRepo, _) = CreateHandler();
        await SeedFreeUserWithDevice(userProfileRepo, deviceRepo);

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert
        await Assert.That(notificationRepo.All).HasCount().EqualTo(1);
        await Assert.That(notificationRepo.All[0].UserId).IsEqualTo("user-1");
        await Assert.That(notificationRepo.All[0].ApplicationName).IsEqualTo("app-001");
        await Assert.That(notificationRepo.All[0].WatchZoneId).IsEqualTo("zone-1");
    }

    [Test]
    public async Task Should_SendPushNotification_When_UserHasRegisteredDevice()
    {
        // Arrange
        var (handler, _, userProfileRepo, pushSender, deviceRepo, _) = CreateHandler();
        await SeedFreeUserWithDevice(userProfileRepo, deviceRepo);

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert
        await Assert.That(pushSender.Sent).HasCount().EqualTo(1);
        await Assert.That(pushSender.Sent[0].Devices).HasCount().EqualTo(1);
        await Assert.That(pushSender.Sent[0].Notification.ApplicationName).IsEqualTo("app-001");
    }

    [Test]
    public async Task Should_MarkPushSent_When_NotificationDispatched()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, _, deviceRepo, _) = CreateHandler();
        await SeedFreeUserWithDevice(userProfileRepo, deviceRepo);

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert
        await Assert.That(notificationRepo.All[0].PushSent).IsTrue();
    }

    [Test]
    public async Task Should_NotSendDuplicateNotification_When_SameApplicationAndUser()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, _) = CreateHandler();
        await SeedFreeUserWithDevice(userProfileRepo, deviceRepo);

        var command = CreateCommand();

        // Act
        await handler.HandleAsync(command, CancellationToken.None);
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(notificationRepo.All).HasCount().EqualTo(1);
        await Assert.That(pushSender.Sent).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_RecordNotificationButNotPush_When_PushDisabled()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, pushSender, _, _) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithPushEnabled(false)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert
        await Assert.That(notificationRepo.All).HasCount().EqualTo(1);
        await Assert.That(notificationRepo.All[0].PushSent).IsFalse();
        await Assert.That(pushSender.Sent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_EnforceFreeTierCap_When_FiveNotificationsSentInMonth()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, _) = CreateHandler();
        await SeedFreeUserWithDevice(userProfileRepo, deviceRepo);

        for (var i = 0; i < 5; i++)
        {
            await handler.HandleAsync(CreateCommand($"app-{i:D3}"), CancellationToken.None);
        }

        // Act — 6th notification
        await handler.HandleAsync(CreateCommand("app-005"), CancellationToken.None);

        // Assert — recorded but no push
        await Assert.That(notificationRepo.All).HasCount().EqualTo(6);
        await Assert.That(notificationRepo.All[5].PushSent).IsFalse();
        await Assert.That(pushSender.Sent).HasCount().EqualTo(5);
    }

    [Test]
    public async Task Should_AllowUnlimitedNotifications_When_ProTier()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, _) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Pro)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var device = DeviceRegistration.Create("user-1", "device-token-1", DevicePlatform.Ios, March2026);
        await deviceRepo.SaveAsync(device, CancellationToken.None);

        for (var i = 0; i < 10; i++)
        {
            await handler.HandleAsync(CreateCommand($"app-{i:D3}"), CancellationToken.None);
        }

        // Assert — all 10 pushed
        await Assert.That(notificationRepo.All).HasCount().EqualTo(10);
        await Assert.That(pushSender.Sent).HasCount().EqualTo(10);

        for (var i = 0; i < 10; i++)
        {
            await Assert.That(notificationRepo.All[i].PushSent).IsTrue();
        }
    }

    [Test]
    public async Task Should_ResetCap_When_NewCalendarMonth()
    {
        // Arrange
        var timeProvider = new FakeTimeProvider(new DateTimeOffset(2026, 3, 28, 10, 0, 0, TimeSpan.Zero));
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, _) = CreateHandler(timeProvider);
        await SeedFreeUserWithDevice(userProfileRepo, deviceRepo);

        for (var i = 0; i < 5; i++)
        {
            await handler.HandleAsync(CreateCommand($"march-{i:D3}"), CancellationToken.None);
        }

        await Assert.That(pushSender.Sent).HasCount().EqualTo(5);

        // Advance to April 1st
        timeProvider.SetUtcNow(new DateTimeOffset(2026, 4, 1, 0, 0, 1, TimeSpan.Zero));

        // Act
        await handler.HandleAsync(CreateCommand("april-001"), CancellationToken.None);

        // Assert — push sent (cap reset)
        await Assert.That(notificationRepo.All).HasCount().EqualTo(6);
        await Assert.That(notificationRepo.All[5].PushSent).IsTrue();
        await Assert.That(pushSender.Sent).HasCount().EqualTo(6);
    }

    [Test]
    public async Task Should_NotCreateNotification_When_UserProfileNotFound()
    {
        // Arrange — no user profile seeded
        var (handler, notificationRepo, _, pushSender, _, _) = CreateHandler();

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert
        await Assert.That(notificationRepo.All).HasCount().EqualTo(0);
        await Assert.That(pushSender.Sent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_NotSendPush_When_NoRegisteredDevices()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, pushSender, _, _) = CreateHandler();

        var profile = new UserProfileBuilder().WithUserId("user-1").Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert
        await Assert.That(notificationRepo.All).HasCount().EqualTo(1);
        await Assert.That(notificationRepo.All[0].PushSent).IsFalse();
        await Assert.That(pushSender.Sent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_RecordButNotPush_When_ZoneNewApplicationsDisabled()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, _) = CreateHandler();

        var profile = new UserProfileBuilder().WithUserId("user-1").Build();
        profile.SetZonePreferences(
            "zone-1",
            new ZoneNotificationPreferences(
                NewApplications: false,
                StatusChanges: false,
                DecisionUpdates: false));
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var device = DeviceRegistration.Create("user-1", "device-token-1", DevicePlatform.Ios, March2026);
        await deviceRepo.SaveAsync(device, CancellationToken.None);

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert — notification recorded but push not sent
        await Assert.That(notificationRepo.All).HasCount().EqualTo(1);
        await Assert.That(notificationRepo.All[0].PushSent).IsFalse();
        await Assert.That(pushSender.Sent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_SendPush_When_ZoneNewApplicationsEnabled()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, _) = CreateHandler();

        var profile = new UserProfileBuilder().WithUserId("user-1").Build();
        profile.SetZonePreferences(
            "zone-1",
            new ZoneNotificationPreferences(
                NewApplications: true,
                StatusChanges: false,
                DecisionUpdates: false));
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var device = DeviceRegistration.Create("user-1", "device-token-1", DevicePlatform.Ios, March2026);
        await deviceRepo.SaveAsync(device, CancellationToken.None);

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert
        await Assert.That(notificationRepo.All).HasCount().EqualTo(1);
        await Assert.That(notificationRepo.All[0].PushSent).IsTrue();
        await Assert.That(pushSender.Sent).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_SendInstantEmail_When_PersonalUserHasEmailInstantEnabled()
    {
        var (handler, _, userProfileRepo, _, deviceRepo, emailSender) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithEmail("user@example.com")
            .WithTier(SubscriptionTier.Personal)
            .WithEmailInstantEnabled(true)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var device = DeviceRegistration.Create("user-1", "device-token-1", DevicePlatform.Ios, March2026);
        await deviceRepo.SaveAsync(device, CancellationToken.None);

        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        await Assert.That(emailSender.NotificationsSent).HasCount().EqualTo(1);
        await Assert.That(emailSender.NotificationsSent[0].Email).IsEqualTo("user@example.com");
    }

    [Test]
    public async Task Should_NotSendInstantEmail_When_FreeUserHasEmailInstantEnabled()
    {
        var (handler, _, userProfileRepo, _, deviceRepo, emailSender) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithEmail("user@example.com")
            .WithEmailInstantEnabled(true)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var device = DeviceRegistration.Create("user-1", "device-token-1", DevicePlatform.Ios, March2026);
        await deviceRepo.SaveAsync(device, CancellationToken.None);

        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        await Assert.That(emailSender.NotificationsSent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_NotSendInstantEmail_When_EmailInstantDisabled()
    {
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

        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        await Assert.That(emailSender.NotificationsSent).HasCount().EqualTo(0);
    }

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

    private static async Task SeedFreeUserWithDevice(
        FakeUserProfileRepository userProfileRepo,
        FakeDeviceRegistrationRepository deviceRepo,
        string userId = "user-1")
    {
        var profile = new UserProfileBuilder()
            .WithUserId(userId)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var device = DeviceRegistration.Create(userId, "device-token-1", DevicePlatform.Ios, March2026);
        await deviceRepo.SaveAsync(device, CancellationToken.None);
    }

    private static DispatchNotificationCommand CreateCommand(string applicationName = "app-001")
    {
        var application = new PlanningApplicationBuilder()
            .WithName(applicationName)
            .WithCoordinates(51.5074, -0.1278)
            .Build();

        var zone = new WatchZoneBuilder()
            .WithUserId("user-1")
            .WithId("zone-1")
            .Build();

        return new DispatchNotificationCommand(application, zone);
    }
}
