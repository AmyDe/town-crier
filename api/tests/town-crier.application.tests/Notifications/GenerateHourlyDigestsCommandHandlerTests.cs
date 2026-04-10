using TownCrier.Application.Notifications;
using TownCrier.Application.Tests.Polling;
using TownCrier.Application.Tests.UserProfiles;
using TownCrier.Domain.Notifications;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.Notifications;

public sealed class GenerateHourlyDigestsCommandHandlerTests
{
    [Test]
    public async Task Should_SendDigestEmail_When_PersonalUserHasUnsentNotifications()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, emailSender, watchZoneRepo) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithEmail("test@example.com")
            .WithTier(SubscriptionTier.Personal)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var zone = new WatchZoneBuilder()
            .WithId("zone-1")
            .WithUserId("user-1")
            .WithName("Home")
            .Build();
        await watchZoneRepo.SaveAsync(zone, CancellationToken.None);

        SeedUnsentNotifications(notificationRepo, "user-1", "zone-1", count: 3);

        // Act
        await handler.HandleAsync(new GenerateHourlyDigestsCommand(), CancellationToken.None);

        // Assert
        await Assert.That(emailSender.DigestsSent).HasCount().EqualTo(1);
        await Assert.That(emailSender.DigestsSent[0].Email).IsEqualTo("test@example.com");
        await Assert.That(emailSender.DigestsSent[0].Digests).HasCount().EqualTo(1);
        await Assert.That(emailSender.DigestsSent[0].Digests[0].WatchZoneName).IsEqualTo("Home");
        await Assert.That(emailSender.DigestsSent[0].Digests[0].Notifications).HasCount().EqualTo(3);
    }

    [Test]
    public async Task Should_SkipUser_When_FreeTier()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, emailSender, _) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithEmail("test@example.com")
            .WithTier(SubscriptionTier.Free)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        SeedUnsentNotifications(notificationRepo, "user-1", "zone-1", count: 3);

        // Act
        await handler.HandleAsync(new GenerateHourlyDigestsCommand(), CancellationToken.None);

        // Assert
        await Assert.That(emailSender.DigestsSent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_SkipUser_When_NoEmail()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, emailSender, _) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Personal)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        SeedUnsentNotifications(notificationRepo, "user-1", "zone-1", count: 3);

        // Act
        await handler.HandleAsync(new GenerateHourlyDigestsCommand(), CancellationToken.None);

        // Assert
        await Assert.That(emailSender.DigestsSent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_SkipUser_When_EmailDigestDisabled()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, emailSender, _) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithEmail("test@example.com")
            .WithTier(SubscriptionTier.Personal)
            .WithEmailDigestEnabled(false)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        SeedUnsentNotifications(notificationRepo, "user-1", "zone-1", count: 3);

        // Act
        await handler.HandleAsync(new GenerateHourlyDigestsCommand(), CancellationToken.None);

        // Assert
        await Assert.That(emailSender.DigestsSent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_CompleteImmediately_When_NoUsersHaveUnsentNotifications()
    {
        // Arrange
        var (handler, _, _, emailSender, _) = CreateHandler();

        // No notifications seeded

        // Act
        await handler.HandleAsync(new GenerateHourlyDigestsCommand(), CancellationToken.None);

        // Assert
        await Assert.That(emailSender.DigestsSent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_GroupNotificationsByWatchZone_When_SendingDigest()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, emailSender, watchZoneRepo) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithEmail("test@example.com")
            .WithTier(SubscriptionTier.Personal)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var zone1 = new WatchZoneBuilder()
            .WithId("zone-1")
            .WithUserId("user-1")
            .WithName("Home")
            .Build();
        var zone2 = new WatchZoneBuilder()
            .WithId("zone-2")
            .WithUserId("user-1")
            .WithName("Office")
            .Build();
        await watchZoneRepo.SaveAsync(zone1, CancellationToken.None);
        await watchZoneRepo.SaveAsync(zone2, CancellationToken.None);

        SeedUnsentNotifications(notificationRepo, "user-1", "zone-1", count: 2);
        SeedUnsentNotifications(notificationRepo, "user-1", "zone-2", count: 3);

        // Act
        await handler.HandleAsync(new GenerateHourlyDigestsCommand(), CancellationToken.None);

        // Assert
        await Assert.That(emailSender.DigestsSent).HasCount().EqualTo(1);
        var digests = emailSender.DigestsSent[0].Digests;
        await Assert.That(digests).HasCount().EqualTo(2);
        await Assert.That(digests.Any(d => d.WatchZoneName == "Home" && d.Notifications.Count == 2)).IsTrue();
        await Assert.That(digests.Any(d => d.WatchZoneName == "Office" && d.Notifications.Count == 3)).IsTrue();
    }

    [Test]
    public async Task Should_MarkNotificationsAsEmailSent_When_DigestSent()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, _, watchZoneRepo) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithEmail("test@example.com")
            .WithTier(SubscriptionTier.Personal)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var zone = new WatchZoneBuilder()
            .WithId("zone-1")
            .WithUserId("user-1")
            .WithName("Home")
            .Build();
        await watchZoneRepo.SaveAsync(zone, CancellationToken.None);

        SeedUnsentNotifications(notificationRepo, "user-1", "zone-1", count: 2);

        // Act
        await handler.HandleAsync(new GenerateHourlyDigestsCommand(), CancellationToken.None);

        // Assert
        var allNotifications = notificationRepo.All;
        await Assert.That(allNotifications.All(n => n.EmailSent)).IsTrue();
    }

    [Test]
    public async Task Should_SendDigestsToMultipleUsers_When_BothHaveUnsentNotifications()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, emailSender, watchZoneRepo) = CreateHandler();

        var profile1 = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithEmail("user1@example.com")
            .WithTier(SubscriptionTier.Personal)
            .Build();
        var profile2 = new UserProfileBuilder()
            .WithUserId("user-2")
            .WithEmail("user2@example.com")
            .WithTier(SubscriptionTier.Pro)
            .Build();
        await userProfileRepo.SaveAsync(profile1, CancellationToken.None);
        await userProfileRepo.SaveAsync(profile2, CancellationToken.None);

        var zone1 = new WatchZoneBuilder()
            .WithId("zone-1")
            .WithUserId("user-1")
            .WithName("Home")
            .Build();
        var zone2 = new WatchZoneBuilder()
            .WithId("zone-2")
            .WithUserId("user-2")
            .WithName("Work")
            .Build();
        await watchZoneRepo.SaveAsync(zone1, CancellationToken.None);
        await watchZoneRepo.SaveAsync(zone2, CancellationToken.None);

        SeedUnsentNotifications(notificationRepo, "user-1", "zone-1", count: 2);
        SeedUnsentNotifications(notificationRepo, "user-2", "zone-2", count: 4);

        // Act
        await handler.HandleAsync(new GenerateHourlyDigestsCommand(), CancellationToken.None);

        // Assert
        await Assert.That(emailSender.DigestsSent).HasCount().EqualTo(2);
    }

    [Test]
    public async Task Should_SkipUser_When_ProfileNotFound()
    {
        // Arrange
        var (handler, notificationRepo, _, emailSender, _) = CreateHandler();

        // Seed notifications but no profile
        SeedUnsentNotifications(notificationRepo, "user-1", "zone-1", count: 3);

        // Act
        await handler.HandleAsync(new GenerateHourlyDigestsCommand(), CancellationToken.None);

        // Assert
        await Assert.That(emailSender.DigestsSent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_NotMarkAsSent_When_UserSkipped()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, _, _) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithEmail("test@example.com")
            .WithTier(SubscriptionTier.Free)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        SeedUnsentNotifications(notificationRepo, "user-1", "zone-1", count: 2);

        // Act
        await handler.HandleAsync(new GenerateHourlyDigestsCommand(), CancellationToken.None);

        // Assert -- notifications should remain unsent
        var allNotifications = notificationRepo.All;
        await Assert.That(allNotifications.All(n => !n.EmailSent)).IsTrue();
    }

    private static (GenerateHourlyDigestsCommandHandler Handler,
        FakeNotificationRepository NotificationRepo,
        FakeUserProfileRepository UserProfileRepo,
        SpyEmailSender EmailSender,
        FakeWatchZoneRepository WatchZoneRepo) CreateHandler()
    {
        var notificationRepo = new FakeNotificationRepository();
        var userProfileRepo = new FakeUserProfileRepository();
        var emailSender = new SpyEmailSender();
        var watchZoneRepo = new FakeWatchZoneRepository();

        var handler = new GenerateHourlyDigestsCommandHandler(
            notificationRepo,
            userProfileRepo,
            emailSender,
            watchZoneRepo);

        return (handler, notificationRepo, userProfileRepo, emailSender, watchZoneRepo);
    }

    private static void SeedUnsentNotifications(
        FakeNotificationRepository notificationRepo,
        string userId,
        string watchZoneId,
        int count)
    {
        for (var i = 0; i < count; i++)
        {
            var notification = Notification.Create(
                userId: userId,
                applicationName: $"APP/{watchZoneId}/{i:D3}",
                watchZoneId: watchZoneId,
                applicationAddress: $"{i} Test Street",
                applicationDescription: "Test application",
                applicationType: "Full",
                authorityId: 1,
                now: DateTimeOffset.UtcNow.AddMinutes(-i));
            notificationRepo.Seed(notification);
        }
    }
}
