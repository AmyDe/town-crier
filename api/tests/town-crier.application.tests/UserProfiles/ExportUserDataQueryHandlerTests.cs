using TownCrier.Application.DecisionAlerts;
using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.Notifications;
using TownCrier.Application.OfferCodes;
using TownCrier.Application.SavedApplications;
using TownCrier.Application.Tests.DecisionAlerts;
using TownCrier.Application.Tests.DeviceRegistrations;
using TownCrier.Application.Tests.Notifications;
using TownCrier.Application.Tests.OfferCodes;
using TownCrier.Application.Tests.Polling;
using TownCrier.Application.Tests.SavedApplications;
using TownCrier.Application.UserProfiles;
using TownCrier.Application.WatchZones;
using TownCrier.Domain.DecisionAlerts;
using TownCrier.Domain.DeviceRegistrations;
using TownCrier.Domain.Geocoding;
using TownCrier.Domain.Notifications;
using TownCrier.Domain.OfferCodes;
using TownCrier.Domain.SavedApplications;
using TownCrier.Domain.UserProfiles;
using TownCrier.Domain.WatchZones;

namespace TownCrier.Application.Tests.UserProfiles;

public sealed class ExportUserDataQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnUserData_When_ProfileExists()
    {
        // Arrange
        var harness = new ExportUserDataHarness();
        var profile = UserProfile.Register("auth0|user1", "user1@example.com");
        profile.UpdatePreferences(new NotificationPreferences(PushEnabled: true));
        await harness.UserProfileRepository.SaveAsync(profile, CancellationToken.None);

        // Act
        var result = await harness.Handler.HandleAsync(
            new ExportUserDataQuery("auth0|user1"), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.UserId).IsEqualTo("auth0|user1");
        await Assert.That(result.Email).IsEqualTo("user1@example.com");
        await Assert.That(result.NotificationPreferences.PushEnabled).IsTrue();
        await Assert.That(result.Subscription.Tier).IsEqualTo(SubscriptionTier.Free);
    }

    [Test]
    public async Task Should_ReturnNull_When_ProfileDoesNotExist()
    {
        // Arrange
        var harness = new ExportUserDataHarness();

        // Act
        var result = await harness.Handler.HandleAsync(
            new ExportUserDataQuery("auth0|nonexistent"), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNull();
    }

    [Test]
    public async Task Should_IncludeNotificationPreferences_When_ProfileExists()
    {
        // Arrange
        var harness = new ExportUserDataHarness();
        var profile = UserProfile.Register("auth0|user1");
        profile.UpdatePreferences(new NotificationPreferences(
            PushEnabled: false,
            DigestDay: DayOfWeek.Friday,
            EmailDigestEnabled: false));
        profile.SetZonePreferences("zone-1", new ZoneNotificationPreferences(
            NewApplications: true,
            StatusChanges: false,
            DecisionUpdates: false));
        await harness.UserProfileRepository.SaveAsync(profile, CancellationToken.None);

        // Act
        var result = await harness.Handler.HandleAsync(
            new ExportUserDataQuery("auth0|user1"), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.NotificationPreferences.PushEnabled).IsFalse();
        await Assert.That(result.NotificationPreferences.DigestDay).IsEqualTo(DayOfWeek.Friday);
        await Assert.That(result.NotificationPreferences.EmailDigestEnabled).IsFalse();
        await Assert.That(result.NotificationPreferences.ZonePreferences).HasCount().EqualTo(1);
        await Assert.That(result.NotificationPreferences.ZonePreferences[0].ZoneId).IsEqualTo("zone-1");
        await Assert.That(result.NotificationPreferences.ZonePreferences[0].NewApplications).IsTrue();
    }

    [Test]
    public async Task Should_IncludeSubscriptionMetadata_When_ProfileExists()
    {
        // Arrange
        var harness = new ExportUserDataHarness();
        var profile = UserProfile.Register("auth0|user1");
        profile.LinkOriginalTransactionId("txn-42");
        profile.ActivateSubscription(SubscriptionTier.Pro, new DateTimeOffset(2026, 6, 1, 0, 0, 0, TimeSpan.Zero));
        await harness.UserProfileRepository.SaveAsync(profile, CancellationToken.None);

        // Act
        var result = await harness.Handler.HandleAsync(
            new ExportUserDataQuery("auth0|user1"), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.Subscription.Tier).IsEqualTo(SubscriptionTier.Pro);
        await Assert.That(result.Subscription.OriginalTransactionId).IsEqualTo("txn-42");
        await Assert.That(result.Subscription.ExpiresAt).IsEqualTo(
            new DateTimeOffset(2026, 6, 1, 0, 0, 0, TimeSpan.Zero));
        await Assert.That(result.Subscription.GracePeriodExpiresAt).IsNull();
    }

    [Test]
    public async Task Should_IncludeWatchZones_OnlyForQueriedUser()
    {
        // Arrange
        var harness = new ExportUserDataHarness();
        await harness.UserProfileRepository.SaveAsync(UserProfile.Register("auth0|user1"), CancellationToken.None);
        harness.WatchZoneRepository.Add(BuildWatchZone("auth0|user1", "zone-1", "Home"));
        harness.WatchZoneRepository.Add(BuildWatchZone("auth0|user1", "zone-2", "Office"));
        harness.WatchZoneRepository.Add(BuildWatchZone("auth0|other", "zone-3", "Other user's zone"));

        // Act
        var result = await harness.Handler.HandleAsync(
            new ExportUserDataQuery("auth0|user1"), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.WatchZones).HasCount().EqualTo(2);
        await Assert.That(result.WatchZones.Any(z => z.Id == "zone-1" && z.Name == "Home")).IsTrue();
        await Assert.That(result.WatchZones.Any(z => z.Id == "zone-2")).IsTrue();
        await Assert.That(result.WatchZones.All(z => z.Id != "zone-3")).IsTrue();
    }

    [Test]
    public async Task Should_IncludeNotifications_OnlyForQueriedUser()
    {
        // Arrange
        var harness = new ExportUserDataHarness();
        await harness.UserProfileRepository.SaveAsync(UserProfile.Register("auth0|user1"), CancellationToken.None);
        harness.NotificationRepository.Seed(BuildNotification("auth0|user1", "app-a"));
        harness.NotificationRepository.Seed(BuildNotification("auth0|user1", "app-b"));
        harness.NotificationRepository.Seed(BuildNotification("auth0|other", "app-c"));

        // Act
        var result = await harness.Handler.HandleAsync(
            new ExportUserDataQuery("auth0|user1"), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.Notifications).HasCount().EqualTo(2);
        await Assert.That(result.Notifications.All(n => n.ApplicationName == "app-a" || n.ApplicationName == "app-b")).IsTrue();
    }

    [Test]
    public async Task Should_IncludeDecisionAlerts_OnlyForQueriedUser()
    {
        // Arrange
        var harness = new ExportUserDataHarness();
        await harness.UserProfileRepository.SaveAsync(UserProfile.Register("auth0|user1"), CancellationToken.None);
        await harness.DecisionAlertRepository.SaveAsync(
            DecisionAlert.Create("auth0|user1", "app-a", "ref-a", "10 Example St", "GRANTED", DateTimeOffset.UtcNow),
            CancellationToken.None);
        await harness.DecisionAlertRepository.SaveAsync(
            DecisionAlert.Create("auth0|user1", "app-b", "ref-b", "20 Example St", "REFUSED", DateTimeOffset.UtcNow),
            CancellationToken.None);
        await harness.DecisionAlertRepository.SaveAsync(
            DecisionAlert.Create("auth0|other", "app-c", "ref-c", "30 Example St", "GRANTED", DateTimeOffset.UtcNow),
            CancellationToken.None);

        // Act
        var result = await harness.Handler.HandleAsync(
            new ExportUserDataQuery("auth0|user1"), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.DecisionAlerts).HasCount().EqualTo(2);
        await Assert.That(result.DecisionAlerts.All(a => a.ApplicationUid == "app-a" || a.ApplicationUid == "app-b")).IsTrue();
    }

    [Test]
    public async Task Should_IncludeSavedApplications_OnlyForQueriedUser()
    {
        // Arrange
        var harness = new ExportUserDataHarness();
        await harness.UserProfileRepository.SaveAsync(UserProfile.Register("auth0|user1"), CancellationToken.None);
        await harness.SavedApplicationRepository.SaveAsync(
            SavedApplication.Create("auth0|user1", "app-a", DateTimeOffset.UtcNow),
            CancellationToken.None);
        await harness.SavedApplicationRepository.SaveAsync(
            SavedApplication.Create("auth0|user1", "app-b", DateTimeOffset.UtcNow),
            CancellationToken.None);
        await harness.SavedApplicationRepository.SaveAsync(
            SavedApplication.Create("auth0|other", "app-c", DateTimeOffset.UtcNow),
            CancellationToken.None);

        // Act
        var result = await harness.Handler.HandleAsync(
            new ExportUserDataQuery("auth0|user1"), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.SavedApplications).HasCount().EqualTo(2);
        await Assert.That(result.SavedApplications.All(s => s.ApplicationUid == "app-a" || s.ApplicationUid == "app-b")).IsTrue();
    }

    [Test]
    public async Task Should_IncludeDeviceRegistrations_OnlyForQueriedUser()
    {
        // Arrange
        var harness = new ExportUserDataHarness();
        await harness.UserProfileRepository.SaveAsync(UserProfile.Register("auth0|user1"), CancellationToken.None);
        await harness.DeviceRegistrationRepository.SaveAsync(
            DeviceRegistration.Create("auth0|user1", "token-a", DevicePlatform.Ios, DateTimeOffset.UtcNow),
            CancellationToken.None);
        await harness.DeviceRegistrationRepository.SaveAsync(
            DeviceRegistration.Create("auth0|user1", "token-b", DevicePlatform.Android, DateTimeOffset.UtcNow),
            CancellationToken.None);
        await harness.DeviceRegistrationRepository.SaveAsync(
            DeviceRegistration.Create("auth0|other", "token-c", DevicePlatform.Ios, DateTimeOffset.UtcNow),
            CancellationToken.None);

        // Act
        var result = await harness.Handler.HandleAsync(
            new ExportUserDataQuery("auth0|user1"), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.DeviceRegistrations).HasCount().EqualTo(2);
        await Assert.That(result.DeviceRegistrations.Any(d => d.Platform == DevicePlatform.Ios)).IsTrue();
        await Assert.That(result.DeviceRegistrations.Any(d => d.Platform == DevicePlatform.Android)).IsTrue();
    }

    [Test]
    public async Task Should_IncludeOfferCodeRedemptions_OnlyForQueriedUser()
    {
        // Arrange
        var harness = new ExportUserDataHarness();
        await harness.UserProfileRepository.SaveAsync(UserProfile.Register("auth0|user1"), CancellationToken.None);

        var codeA = new OfferCode("AAAAAAAAAAAA", SubscriptionTier.Pro, 30, DateTimeOffset.UtcNow);
        codeA.Redeem("auth0|user1", DateTimeOffset.UtcNow);
        await harness.OfferCodeRepository.SaveAsync(codeA, CancellationToken.None);

        var codeB = new OfferCode("BBBBBBBBBBBB", SubscriptionTier.Personal, 7, DateTimeOffset.UtcNow);
        codeB.Redeem("auth0|other", DateTimeOffset.UtcNow);
        await harness.OfferCodeRepository.SaveAsync(codeB, CancellationToken.None);

        var codeC = new OfferCode("CCCCCCCCCCCC", SubscriptionTier.Pro, 14, DateTimeOffset.UtcNow);
        // Not redeemed.
        await harness.OfferCodeRepository.SaveAsync(codeC, CancellationToken.None);

        // Act
        var result = await harness.Handler.HandleAsync(
            new ExportUserDataQuery("auth0|user1"), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.OfferCodeRedemptions).HasCount().EqualTo(1);
        await Assert.That(result.OfferCodeRedemptions[0].Code).IsEqualTo("AAAAAAAAAAAA");
        await Assert.That(result.OfferCodeRedemptions[0].Tier).IsEqualTo(SubscriptionTier.Pro);
        await Assert.That(result.OfferCodeRedemptions[0].DurationDays).IsEqualTo(30);
    }

    [Test]
    public async Task Should_ReturnEmptyCollections_When_ProfileHasNoChildRecords()
    {
        // Arrange
        var harness = new ExportUserDataHarness();
        await harness.UserProfileRepository.SaveAsync(UserProfile.Register("auth0|user1"), CancellationToken.None);

        // Act
        var result = await harness.Handler.HandleAsync(
            new ExportUserDataQuery("auth0|user1"), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.WatchZones).IsEmpty();
        await Assert.That(result.Notifications).IsEmpty();
        await Assert.That(result.DecisionAlerts).IsEmpty();
        await Assert.That(result.SavedApplications).IsEmpty();
        await Assert.That(result.DeviceRegistrations).IsEmpty();
        await Assert.That(result.OfferCodeRedemptions).IsEmpty();
        await Assert.That(result.NotificationPreferences.ZonePreferences).IsEmpty();
    }

    private static Notification BuildNotification(string userId, string applicationName)
    {
        return Notification.Create(
            userId: userId,
            applicationName: applicationName,
            watchZoneId: "zone-1",
            applicationAddress: "10 Example St",
            applicationDescription: "Extension",
            applicationType: "HouseholderApplication",
            authorityId: 1,
            now: DateTimeOffset.UtcNow);
    }

    private static WatchZone BuildWatchZone(string userId, string zoneId, string name)
    {
        return new WatchZone(
            id: zoneId,
            userId: userId,
            name: name,
            centre: new Coordinates(51.5, -0.1),
            radiusMetres: 500,
            authorityId: 1,
            createdAt: DateTimeOffset.UtcNow);
    }

    private sealed class ExportUserDataHarness
    {
        public FakeUserProfileRepository UserProfileRepository { get; } = new();

        public FakeWatchZoneRepository WatchZoneRepository { get; } = new();

        public FakeNotificationRepository NotificationRepository { get; } = new();

        public FakeDecisionAlertRepository DecisionAlertRepository { get; } = new();

        public FakeSavedApplicationRepository SavedApplicationRepository { get; } = new();

        public FakeDeviceRegistrationRepository DeviceRegistrationRepository { get; } = new();

        public FakeOfferCodeRepository OfferCodeRepository { get; } = new();

        public ExportUserDataQueryHandler Handler => new(
            this.UserProfileRepository,
            this.WatchZoneRepository,
            this.NotificationRepository,
            this.DecisionAlertRepository,
            this.SavedApplicationRepository,
            this.DeviceRegistrationRepository,
            this.OfferCodeRepository);
    }
}
