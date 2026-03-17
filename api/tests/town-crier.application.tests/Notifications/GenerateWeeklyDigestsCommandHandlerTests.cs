using TownCrier.Application.Notifications;
using TownCrier.Application.Tests.DeviceRegistrations;
using TownCrier.Application.Tests.UserProfiles;
using TownCrier.Domain.DeviceRegistrations;
using TownCrier.Domain.Notifications;
using TownCrier.Domain.UserProfiles;
using FakeTimeProvider = TownCrier.Application.Tests.DeviceRegistrations.FakeTimeProvider;

namespace TownCrier.Application.Tests.Notifications;

public sealed class GenerateWeeklyDigestsCommandHandlerTests
{
    // Monday 2026-03-16 at 08:00 UTC
    private static readonly DateTimeOffset MondayMarch2026 = new(2026, 3, 16, 8, 0, 0, TimeSpan.Zero);

    [Test]
    public async Task Should_SendDigestPush_When_ProUserHasNotificationsThisWeek()
    {
        // Arrange
        var timeProvider = new FakeTimeProvider(MondayMarch2026);
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo) =
            CreateHandler(timeProvider);

        await SeedProUserWithDevice(userProfileRepo, deviceRepo, "user-1");
        SeedNotifications(notificationRepo, "user-1", count: 3, createdAt: MondayMarch2026.AddDays(-2));

        // Act
        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        // Assert
        await Assert.That(pushSender.DigestsSent).HasCount().EqualTo(1);
        await Assert.That(pushSender.DigestsSent[0].ApplicationCount).IsEqualTo(3);
        await Assert.That(pushSender.DigestsSent[0].Devices).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_NotSendDigest_When_UserIsFreesTier()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Free)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var device = DeviceRegistration.Create("user-1", "device-1", DevicePlatform.Ios, MondayMarch2026);
        await deviceRepo.SaveAsync(device, CancellationToken.None);
        SeedNotifications(notificationRepo, "user-1", count: 5, createdAt: MondayMarch2026.AddDays(-1));

        // Act
        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        // Assert
        await Assert.That(pushSender.DigestsSent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_NotSendDigest_When_ZeroNewApplications()
    {
        // Arrange
        var (handler, _, userProfileRepo, pushSender, deviceRepo) = CreateHandler();
        await SeedProUserWithDevice(userProfileRepo, deviceRepo, "user-1");

        // No notifications seeded

        // Act
        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        // Assert
        await Assert.That(pushSender.DigestsSent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_OnlySendDigest_When_DigestDayMatchesToday()
    {
        // Arrange — today is Monday, user digest day is Wednesday
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo) = CreateHandler();
        await SeedProUserWithDevice(userProfileRepo, deviceRepo, "user-1", digestDay: DayOfWeek.Wednesday);
        SeedNotifications(notificationRepo, "user-1", count: 3, createdAt: MondayMarch2026.AddDays(-2));

        // Act
        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        // Assert
        await Assert.That(pushSender.DigestsSent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_NotSendDigest_When_PushDisabled()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Pro)
            .WithPushEnabled(false)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var device = DeviceRegistration.Create("user-1", "device-1", DevicePlatform.Ios, MondayMarch2026);
        await deviceRepo.SaveAsync(device, CancellationToken.None);
        SeedNotifications(notificationRepo, "user-1", count: 3, createdAt: MondayMarch2026.AddDays(-2));

        // Act
        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        // Assert
        await Assert.That(pushSender.DigestsSent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_NotSendDigest_When_NoRegisteredDevices()
    {
        // Arrange — Pro user, no device registered
        var (handler, notificationRepo, userProfileRepo, pushSender, _) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Pro)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);
        SeedNotifications(notificationRepo, "user-1", count: 3, createdAt: MondayMarch2026.AddDays(-2));

        // Act
        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        // Assert
        await Assert.That(pushSender.DigestsSent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_ExcludeOldNotifications_When_OlderThanSevenDays()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo) = CreateHandler();
        await SeedProUserWithDevice(userProfileRepo, deviceRepo, "user-1");

        // Seed notifications older than 7 days
        SeedNotifications(notificationRepo, "user-1", count: 5, createdAt: MondayMarch2026.AddDays(-10));

        // Act
        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        // Assert — no digest because all notifications are too old
        await Assert.That(pushSender.DigestsSent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_SendDigestsToMultipleProUsers_When_EachHasNotifications()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo) = CreateHandler();
        await SeedProUserWithDevice(userProfileRepo, deviceRepo, "user-1");
        await SeedProUserWithDevice(userProfileRepo, deviceRepo, "user-2");

        SeedNotifications(notificationRepo, "user-1", count: 2, createdAt: MondayMarch2026.AddDays(-1));
        SeedNotifications(notificationRepo, "user-2", count: 5, createdAt: MondayMarch2026.AddDays(-3));

        // Act
        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        // Assert
        await Assert.That(pushSender.DigestsSent).HasCount().EqualTo(2);
    }

    [Test]
    public async Task Should_DefaultDigestDay_When_UsingDefaultPreferences()
    {
        // Arrange — user with default preferences (digest day = Monday), today is Monday
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Pro)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var device = DeviceRegistration.Create("user-1", "device-1", DevicePlatform.Ios, MondayMarch2026);
        await deviceRepo.SaveAsync(device, CancellationToken.None);
        SeedNotifications(notificationRepo, "user-1", count: 2, createdAt: MondayMarch2026.AddDays(-1));

        // Act
        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        // Assert — should send because default digest day is Monday and today is Monday
        await Assert.That(pushSender.DigestsSent).HasCount().EqualTo(1);
    }

    private static (GenerateWeeklyDigestsCommandHandler Handler,
        FakeNotificationRepository NotificationRepo,
        FakeUserProfileRepository UserProfileRepo,
        SpyPushNotificationSender PushSender,
        FakeDeviceRegistrationRepository DeviceRepo) CreateHandler(FakeTimeProvider? timeProvider = null)
    {
        var notificationRepo = new FakeNotificationRepository();
        var userProfileRepo = new FakeUserProfileRepository();
        var deviceRepo = new FakeDeviceRegistrationRepository();
        var pushSender = new SpyPushNotificationSender();
        var tp = timeProvider ?? new FakeTimeProvider(MondayMarch2026);

        var handler = new GenerateWeeklyDigestsCommandHandler(
            userProfileRepo, notificationRepo, deviceRepo, pushSender, tp);

        return (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo);
    }

    private static async Task SeedProUserWithDevice(
        FakeUserProfileRepository userProfileRepo,
        FakeDeviceRegistrationRepository deviceRepo,
        string userId,
        DayOfWeek digestDay = DayOfWeek.Monday)
    {
        var profile = new UserProfileBuilder()
            .WithUserId(userId)
            .WithTier(SubscriptionTier.Pro)
            .WithDigestDay(digestDay)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var device = DeviceRegistration.Create(userId, $"device-{userId}", DevicePlatform.Ios, MondayMarch2026);
        await deviceRepo.SaveAsync(device, CancellationToken.None);
    }

    private static void SeedNotifications(
        FakeNotificationRepository notificationRepo,
        string userId,
        int count,
        DateTimeOffset createdAt)
    {
        for (var i = 0; i < count; i++)
        {
            var notification = Notification.Create(
                userId: userId,
                applicationName: $"app-{i:D3}",
                watchZoneId: "zone-1",
                applicationAddress: $"{i} Test Street",
                applicationDescription: "Test application",
                applicationType: "Full",
                authorityId: 1,
                now: createdAt);
            notificationRepo.Seed(notification);
        }
    }
}
