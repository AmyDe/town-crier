using TownCrier.Application.DecisionAlerts;
using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.Notifications;
using TownCrier.Application.SavedApplications;
using TownCrier.Application.Tests.Admin;
using TownCrier.Application.Tests.DecisionAlerts;
using TownCrier.Application.Tests.DeviceRegistrations;
using TownCrier.Application.Tests.Notifications;
using TownCrier.Application.Tests.Polling;
using TownCrier.Application.Tests.SavedApplications;
using TownCrier.Application.UserProfiles;
using TownCrier.Application.WatchZones;
using TownCrier.Domain.DecisionAlerts;
using TownCrier.Domain.DeviceRegistrations;
using TownCrier.Domain.Geocoding;
using TownCrier.Domain.Notifications;
using TownCrier.Domain.SavedApplications;
using TownCrier.Domain.UserProfiles;
using TownCrier.Domain.WatchZones;

namespace TownCrier.Application.Tests.UserProfiles;

public sealed class DeleteUserProfileCommandHandlerTests
{
    [Test]
    public async Task Should_DeleteProfile_When_ProfileExists()
    {
        // Arrange
        var harness = new DeleteUserProfileHarness();
        var profile = UserProfile.Register("auth0|user1");
        await harness.Repository.SaveAsync(profile, CancellationToken.None);

        // Act
        await harness.Handler.HandleAsync(
            new DeleteUserProfileCommand("auth0|user1"), CancellationToken.None);

        // Assert
        await Assert.That(harness.Repository.GetByUserId("auth0|user1")).IsNull();
    }

    [Test]
    public async Task Should_ThrowNotFound_When_ProfileDoesNotExist()
    {
        // Arrange
        var harness = new DeleteUserProfileHarness();

        // Act & Assert
        await Assert.ThrowsAsync<UserProfileNotFoundException>(
            () => harness.Handler.HandleAsync(
                new DeleteUserProfileCommand("auth0|nonexistent"), CancellationToken.None));
    }

    [Test]
    public async Task Should_NotAffectOtherProfiles_When_DeletingOne()
    {
        // Arrange
        var harness = new DeleteUserProfileHarness();
        await harness.Repository.SaveAsync(UserProfile.Register("auth0|user1"), CancellationToken.None);
        await harness.Repository.SaveAsync(UserProfile.Register("auth0|user2"), CancellationToken.None);

        // Act
        await harness.Handler.HandleAsync(
            new DeleteUserProfileCommand("auth0|user1"), CancellationToken.None);

        // Assert
        await Assert.That(harness.Repository.GetByUserId("auth0|user1")).IsNull();
        await Assert.That(harness.Repository.GetByUserId("auth0|user2")).IsNotNull();
        await Assert.That(harness.Repository.Count).IsEqualTo(1);
    }

    [Test]
    public async Task Should_DeleteAuth0User_When_ProfileExists()
    {
        // Arrange
        var harness = new DeleteUserProfileHarness();
        var profile = UserProfile.Register("auth0|user1");
        await harness.Repository.SaveAsync(profile, CancellationToken.None);

        // Act
        await harness.Handler.HandleAsync(
            new DeleteUserProfileCommand("auth0|user1"), CancellationToken.None);

        // Assert
        await Assert.That(harness.Auth0Client.Deletions).HasCount().EqualTo(1);
        await Assert.That(harness.Auth0Client.Deletions[0]).IsEqualTo("auth0|user1");
    }

    [Test]
    public async Task Should_NotCallAuth0_When_ProfileDoesNotExist()
    {
        // Arrange
        var harness = new DeleteUserProfileHarness();

        // Act
        try
        {
            await harness.Handler.HandleAsync(
                new DeleteUserProfileCommand("auth0|nonexistent"), CancellationToken.None);
        }
        catch (UserProfileNotFoundException)
        {
            // Expected — profile was missing.
        }

        // Assert
        await Assert.That(harness.Auth0Client.Deletions).IsEmpty();
    }

    [Test]
    public async Task Should_CascadeDeleteNotifications_When_ProfileIsDeleted()
    {
        // Arrange
        var harness = new DeleteUserProfileHarness();
        await harness.Repository.SaveAsync(UserProfile.Register("auth0|user1"), CancellationToken.None);
        await harness.Repository.SaveAsync(UserProfile.Register("auth0|user2"), CancellationToken.None);

        harness.NotificationRepository.Seed(BuildNotification("auth0|user1", "app-a"));
        harness.NotificationRepository.Seed(BuildNotification("auth0|user1", "app-b"));
        harness.NotificationRepository.Seed(BuildNotification("auth0|user2", "app-c"));

        // Act
        await harness.Handler.HandleAsync(
            new DeleteUserProfileCommand("auth0|user1"), CancellationToken.None);

        // Assert
        var remaining = harness.NotificationRepository.All;
        await Assert.That(remaining).HasCount().EqualTo(1);
        await Assert.That(remaining[0].UserId).IsEqualTo("auth0|user2");
    }

    [Test]
    public async Task Should_CascadeDeleteDecisionAlerts_When_ProfileIsDeleted()
    {
        // Arrange
        var harness = new DeleteUserProfileHarness();
        await harness.Repository.SaveAsync(UserProfile.Register("auth0|user1"), CancellationToken.None);
        await harness.Repository.SaveAsync(UserProfile.Register("auth0|user2"), CancellationToken.None);

        await harness.DecisionAlertRepository.SaveAsync(
            DecisionAlert.Create("auth0|user1", "app-a", "ref-a", "10 Example St", "GRANTED", DateTimeOffset.UtcNow),
            CancellationToken.None);
        await harness.DecisionAlertRepository.SaveAsync(
            DecisionAlert.Create("auth0|user2", "app-b", "ref-b", "20 Example St", "GRANTED", DateTimeOffset.UtcNow),
            CancellationToken.None);

        // Act
        await harness.Handler.HandleAsync(
            new DeleteUserProfileCommand("auth0|user1"), CancellationToken.None);

        // Assert
        var remaining = harness.DecisionAlertRepository.All;
        await Assert.That(remaining).HasCount().EqualTo(1);
        await Assert.That(remaining[0].UserId).IsEqualTo("auth0|user2");
    }

    [Test]
    public async Task Should_CascadeDeleteWatchZones_When_ProfileIsDeleted()
    {
        // Arrange
        var harness = new DeleteUserProfileHarness();
        await harness.Repository.SaveAsync(UserProfile.Register("auth0|user1"), CancellationToken.None);
        await harness.Repository.SaveAsync(UserProfile.Register("auth0|user2"), CancellationToken.None);

        harness.WatchZoneRepository.Add(BuildWatchZone("auth0|user1", "zone-1"));
        harness.WatchZoneRepository.Add(BuildWatchZone("auth0|user1", "zone-2"));
        harness.WatchZoneRepository.Add(BuildWatchZone("auth0|user2", "zone-3"));

        // Act
        await harness.Handler.HandleAsync(
            new DeleteUserProfileCommand("auth0|user1"), CancellationToken.None);

        // Assert
        var user1Zones = await harness.WatchZoneRepository.GetByUserIdAsync("auth0|user1", CancellationToken.None);
        var user2Zones = await harness.WatchZoneRepository.GetByUserIdAsync("auth0|user2", CancellationToken.None);
        await Assert.That(user1Zones).IsEmpty();
        await Assert.That(user2Zones).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_CascadeDeleteSavedApplications_When_ProfileIsDeleted()
    {
        // Arrange
        var harness = new DeleteUserProfileHarness();
        await harness.Repository.SaveAsync(UserProfile.Register("auth0|user1"), CancellationToken.None);
        await harness.Repository.SaveAsync(UserProfile.Register("auth0|user2"), CancellationToken.None);

        await harness.SavedApplicationRepository.SaveAsync(
            SavedApplication.Create("auth0|user1", "app-a", DateTimeOffset.UtcNow),
            CancellationToken.None);
        await harness.SavedApplicationRepository.SaveAsync(
            SavedApplication.Create("auth0|user1", "app-b", DateTimeOffset.UtcNow),
            CancellationToken.None);
        await harness.SavedApplicationRepository.SaveAsync(
            SavedApplication.Create("auth0|user2", "app-c", DateTimeOffset.UtcNow),
            CancellationToken.None);

        // Act
        await harness.Handler.HandleAsync(
            new DeleteUserProfileCommand("auth0|user1"), CancellationToken.None);

        // Assert
        var user1Saved = await harness.SavedApplicationRepository.GetByUserIdAsync("auth0|user1", CancellationToken.None);
        var user2Saved = await harness.SavedApplicationRepository.GetByUserIdAsync("auth0|user2", CancellationToken.None);
        await Assert.That(user1Saved).IsEmpty();
        await Assert.That(user2Saved).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_CascadeDeleteDeviceRegistrations_When_ProfileIsDeleted()
    {
        // Arrange
        var harness = new DeleteUserProfileHarness();
        await harness.Repository.SaveAsync(UserProfile.Register("auth0|user1"), CancellationToken.None);
        await harness.Repository.SaveAsync(UserProfile.Register("auth0|user2"), CancellationToken.None);

        await harness.DeviceRegistrationRepository.SaveAsync(
            DeviceRegistration.Create("auth0|user1", "token-a", DevicePlatform.Ios, DateTimeOffset.UtcNow),
            CancellationToken.None);
        await harness.DeviceRegistrationRepository.SaveAsync(
            DeviceRegistration.Create("auth0|user1", "token-b", DevicePlatform.Ios, DateTimeOffset.UtcNow),
            CancellationToken.None);
        await harness.DeviceRegistrationRepository.SaveAsync(
            DeviceRegistration.Create("auth0|user2", "token-c", DevicePlatform.Ios, DateTimeOffset.UtcNow),
            CancellationToken.None);

        // Act
        await harness.Handler.HandleAsync(
            new DeleteUserProfileCommand("auth0|user1"), CancellationToken.None);

        // Assert
        var user1Devices = await harness.DeviceRegistrationRepository.GetByUserIdAsync("auth0|user1", CancellationToken.None);
        var user2Devices = await harness.DeviceRegistrationRepository.GetByUserIdAsync("auth0|user2", CancellationToken.None);
        await Assert.That(user1Devices).IsEmpty();
        await Assert.That(user2Devices).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_NotCascadeDelete_When_ProfileDoesNotExist()
    {
        // Arrange — seed child records for a different user; the missing profile must not affect them.
        var harness = new DeleteUserProfileHarness();
        await harness.Repository.SaveAsync(UserProfile.Register("auth0|user2"), CancellationToken.None);
        harness.NotificationRepository.Seed(BuildNotification("auth0|user2", "app-c"));
        harness.WatchZoneRepository.Add(BuildWatchZone("auth0|user2", "zone-2"));

        // Act
        try
        {
            await harness.Handler.HandleAsync(
                new DeleteUserProfileCommand("auth0|missing"), CancellationToken.None);
        }
        catch (UserProfileNotFoundException)
        {
            // Expected.
        }

        // Assert — user2 child records are untouched.
        await Assert.That(harness.NotificationRepository.All).HasCount().EqualTo(1);
        var user2Zones = await harness.WatchZoneRepository.GetByUserIdAsync("auth0|user2", CancellationToken.None);
        await Assert.That(user2Zones).HasCount().EqualTo(1);
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

    private static WatchZone BuildWatchZone(string userId, string zoneId)
    {
        return new WatchZone(
            id: zoneId,
            userId: userId,
            name: "Zone",
            centre: new Coordinates(51.5, -0.1),
            radiusMetres: 500,
            authorityId: 1,
            createdAt: DateTimeOffset.UtcNow);
    }

    private sealed class DeleteUserProfileHarness
    {
        public FakeUserProfileRepository Repository { get; } = new();

        public FakeAuth0ManagementClient Auth0Client { get; } = new();

        public FakeNotificationRepository NotificationRepository { get; } = new();

        public FakeDecisionAlertRepository DecisionAlertRepository { get; } = new();

        public FakeWatchZoneRepository WatchZoneRepository { get; } = new();

        public FakeSavedApplicationRepository SavedApplicationRepository { get; } = new();

        public FakeDeviceRegistrationRepository DeviceRegistrationRepository { get; } = new();

        public DeleteUserProfileCommandHandler Handler => new(
            this.Repository,
            this.Auth0Client,
            this.NotificationRepository,
            this.DecisionAlertRepository,
            this.WatchZoneRepository,
            this.SavedApplicationRepository,
            this.DeviceRegistrationRepository);
    }
}
