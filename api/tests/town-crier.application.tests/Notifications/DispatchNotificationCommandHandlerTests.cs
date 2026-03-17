using Microsoft.Extensions.Time.Testing;
using TownCrier.Application.Notifications;
using TownCrier.Application.Tests.DeviceRegistrations;
using TownCrier.Application.Tests.Polling;
using TownCrier.Application.Tests.UserProfiles;
using TownCrier.Domain.DeviceRegistrations;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.Notifications;

public sealed class DispatchNotificationCommandHandlerTests
{
    private static readonly DateTimeOffset March2026 = new(2026, 3, 15, 10, 0, 0, TimeSpan.Zero);

    [Test]
    public async Task Should_CreateNotificationRecord_When_ApplicationMatchesWatchZone()
    {
        // Arrange
        var (handler, notificationRepo, _, _, _) = CreateHandler();

        await SeedUserWithDevice(handler);

        var command = CreateCommand();

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

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
        var (handler, _, _, pushSender, _) = CreateHandler();

        await SeedUserWithDevice(handler);

        var command = CreateCommand();

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(pushSender.Sent).HasCount().EqualTo(1);
        await Assert.That(pushSender.Sent[0].Devices).HasCount().EqualTo(1);
        await Assert.That(pushSender.Sent[0].Notification.ApplicationName).IsEqualTo("app-001");
    }

    [Test]
    public async Task Should_MarkPushSent_When_NotificationDispatched()
    {
        // Arrange
        var (handler, notificationRepo, _, _, _) = CreateHandler();

        await SeedUserWithDevice(handler);

        var command = CreateCommand();

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(notificationRepo.All[0].PushSent).IsTrue();
    }

    [Test]
    public async Task Should_NotSendDuplicateNotification_When_SameApplicationAndUser()
    {
        // Arrange
        var (handler, notificationRepo, _, pushSender, _) = CreateHandler();

        await SeedUserWithDevice(handler);

        var command = CreateCommand();

        // Act — dispatch twice
        await handler.HandleAsync(command, CancellationToken.None);
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert — only one notification created and one push sent
        await Assert.That(notificationRepo.All).HasCount().EqualTo(1);
        await Assert.That(pushSender.Sent).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_RecordNotificationButNotPush_When_PushDisabled()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, pushSender, _) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithPushEnabled(false)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var command = CreateCommand();

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert — notification recorded but no push
        await Assert.That(notificationRepo.All).HasCount().EqualTo(1);
        await Assert.That(notificationRepo.All[0].PushSent).IsFalse();
        await Assert.That(pushSender.Sent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_EnforceFreeTierCap_When_FiveNotificationsSentInMonth()
    {
        // Arrange
        var (handler, notificationRepo, _, pushSender, _) = CreateHandler();

        await SeedUserWithDevice(handler);

        // Send 5 notifications (the cap)
        for (var i = 0; i < 5; i++)
        {
            var cmd = CreateCommand(applicationName: $"app-{i:D3}");
            await handler.HandleAsync(cmd, CancellationToken.None);
        }

        // Act — 6th notification
        var sixthCommand = CreateCommand(applicationName: "app-005");
        await handler.HandleAsync(sixthCommand, CancellationToken.None);

        // Assert — 6th notification recorded but no push sent
        await Assert.That(notificationRepo.All).HasCount().EqualTo(6);
        await Assert.That(notificationRepo.All[5].PushSent).IsFalse();
        await Assert.That(pushSender.Sent).HasCount().EqualTo(5);
    }

    [Test]
    public async Task Should_AllowUnlimitedNotifications_When_ProTier()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Pro)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var device = DeviceRegistration.Create("user-1", "device-token-1", DevicePlatform.Ios, March2026);
        await deviceRepo.SaveAsync(device, CancellationToken.None);

        // Send 10 notifications (well over the free cap)
        for (var i = 0; i < 10; i++)
        {
            var cmd = CreateCommand(applicationName: $"app-{i:D3}");
            await handler.HandleAsync(cmd, CancellationToken.None);
        }

        // Assert — all 10 have push sent
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
        var (handler, notificationRepo, _, pushSender, _) = CreateHandler(timeProvider);

        await SeedUserWithDevice(handler, timeProvider: timeProvider);

        // Exhaust cap in March
        for (var i = 0; i < 5; i++)
        {
            var cmd = CreateCommand(applicationName: $"march-{i:D3}");
            await handler.HandleAsync(cmd, CancellationToken.None);
        }

        await Assert.That(pushSender.Sent).HasCount().EqualTo(5);

        // Advance to April 1st
        timeProvider.SetUtcNow(new DateTimeOffset(2026, 4, 1, 0, 0, 1, TimeSpan.Zero));

        // Act — first notification in April
        var aprilCommand = CreateCommand(applicationName: "april-001");
        await handler.HandleAsync(aprilCommand, CancellationToken.None);

        // Assert — push sent (cap reset)
        await Assert.That(notificationRepo.All).HasCount().EqualTo(6);
        await Assert.That(notificationRepo.All[5].PushSent).IsTrue();
        await Assert.That(pushSender.Sent).HasCount().EqualTo(6);
    }

    [Test]
    public async Task Should_NotCreateNotification_When_UserProfileNotFound()
    {
        // Arrange — no user profile seeded
        var (handler, notificationRepo, _, pushSender, _) = CreateHandler();

        var command = CreateCommand();

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(notificationRepo.All).HasCount().EqualTo(0);
        await Assert.That(pushSender.Sent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_NotSendPush_When_NoRegisteredDevices()
    {
        // Arrange — user profile but no device
        var (handler, notificationRepo, userProfileRepo, pushSender, _) = CreateHandler();

        var profile = new UserProfileBuilder().WithUserId("user-1").Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var command = CreateCommand();

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert — notification created but push not sent
        await Assert.That(notificationRepo.All).HasCount().EqualTo(1);
        await Assert.That(notificationRepo.All[0].PushSent).IsFalse();
        await Assert.That(pushSender.Sent).HasCount().EqualTo(0);
    }

    private static (DispatchNotificationCommandHandler Handler,
        FakeNotificationRepository NotificationRepo,
        FakeUserProfileRepository UserProfileRepo,
        SpyPushNotificationSender PushSender,
        FakeDeviceRegistrationRepository DeviceRepo) CreateHandler(FakeTimeProvider? timeProvider = null)
    {
        var notificationRepo = new FakeNotificationRepository();
        var userProfileRepo = new FakeUserProfileRepository();
        var deviceRepo = new FakeDeviceRegistrationRepository();
        var pushSender = new SpyPushNotificationSender();
        var tp = timeProvider ?? new FakeTimeProvider(March2026);

        var handler = new DispatchNotificationCommandHandler(
            notificationRepo, userProfileRepo, deviceRepo, pushSender, tp);

        return (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo);
    }

    private static async Task SeedUserWithDevice(
        DispatchNotificationCommandHandler handler,
        string userId = "user-1",
        SubscriptionTier tier = SubscriptionTier.Free,
        FakeTimeProvider? timeProvider = null)
    {
        _ = handler; // used to find the tuple members via deconstruction at call site

        var tp = timeProvider ?? new FakeTimeProvider(March2026);

        var profile = new UserProfileBuilder()
            .WithUserId(userId)
            .WithTier(tier)
            .Build();

        var device = DeviceRegistration.Create(userId, "device-token-1", DevicePlatform.Ios, tp.GetUtcNow());

        // Access the repos through the handler's tuple return — caller provides them
        throw new InvalidOperationException("Use the tuple destructuring pattern instead");
    }

    private static DispatchNotificationCommand CreateCommand(string applicationName = "app-001", string userId = "user-1")
    {
        var application = new PlanningApplicationBuilder()
            .WithName(applicationName)
            .WithCoordinates(51.5074, -0.1278)
            .Build();

        var zone = new WatchZoneBuilder()
            .WithUserId(userId)
            .WithId("zone-1")
            .Build();

        return new DispatchNotificationCommand(application, zone);
    }
}
