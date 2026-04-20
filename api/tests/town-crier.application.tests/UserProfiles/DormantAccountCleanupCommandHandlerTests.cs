using TownCrier.Application.Tests.Admin;
using TownCrier.Application.Tests.DecisionAlerts;
using TownCrier.Application.Tests.DeviceRegistrations;
using TownCrier.Application.Tests.Notifications;
using TownCrier.Application.Tests.Polling;
using TownCrier.Application.Tests.SavedApplications;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.UserProfiles;

public sealed class DormantAccountCleanupCommandHandlerTests
{
    private static readonly DateTimeOffset Now = new(2026, 4, 20, 0, 0, 0, TimeSpan.Zero);

    [Test]
    public async Task Should_DeleteProfile_When_LastActiveBeforeCutoff()
    {
        // Arrange
        var harness = new CleanupHarness();
        var dormantLastActive = Now.AddMonths(-13);
        var profile = UserProfile.Register("auth0|dormant", email: null, now: dormantLastActive);
        await harness.Repository.SaveAsync(profile, CancellationToken.None);

        // Act
        var result = await harness.Handler.HandleAsync(
            new DormantAccountCleanupCommand(Now), CancellationToken.None);

        // Assert
        await Assert.That(result.DeletedCount).IsEqualTo(1);
        await Assert.That(harness.Repository.GetByUserId("auth0|dormant")).IsNull();
    }

    [Test]
    public async Task Should_NotDeleteProfile_When_LastActiveAfterCutoff()
    {
        // Arrange — active within the 12-month retention window.
        var harness = new CleanupHarness();
        var activeLastActive = Now.AddMonths(-6);
        var profile = UserProfile.Register("auth0|active", email: null, now: activeLastActive);
        await harness.Repository.SaveAsync(profile, CancellationToken.None);

        // Act
        var result = await harness.Handler.HandleAsync(
            new DormantAccountCleanupCommand(Now), CancellationToken.None);

        // Assert
        await Assert.That(result.DeletedCount).IsEqualTo(0);
        await Assert.That(harness.Repository.GetByUserId("auth0|active")).IsNotNull();
    }

    [Test]
    public async Task Should_DeleteOnlyDormantProfiles_When_MixedPopulation()
    {
        // Arrange
        var harness = new CleanupHarness();
        await harness.Repository.SaveAsync(
            UserProfile.Register("auth0|dormant-1", email: null, now: Now.AddYears(-2)),
            CancellationToken.None);
        await harness.Repository.SaveAsync(
            UserProfile.Register("auth0|dormant-2", email: null, now: Now.AddMonths(-15)),
            CancellationToken.None);
        await harness.Repository.SaveAsync(
            UserProfile.Register("auth0|active", email: null, now: Now.AddDays(-1)),
            CancellationToken.None);

        // Act
        var result = await harness.Handler.HandleAsync(
            new DormantAccountCleanupCommand(Now), CancellationToken.None);

        // Assert
        await Assert.That(result.DeletedCount).IsEqualTo(2);
        await Assert.That(harness.Repository.GetByUserId("auth0|dormant-1")).IsNull();
        await Assert.That(harness.Repository.GetByUserId("auth0|dormant-2")).IsNull();
        await Assert.That(harness.Repository.GetByUserId("auth0|active")).IsNotNull();
    }

    [Test]
    public async Task Should_CascadeChildRecords_When_DeletingDormantProfile()
    {
        // Arrange
        var harness = new CleanupHarness();
        await harness.Repository.SaveAsync(
            UserProfile.Register("auth0|dormant", email: null, now: Now.AddYears(-2)),
            CancellationToken.None);

        // Seed any cascade target — verifies the cleanup reuses the delete cascade.
        harness.NotificationRepository.Seed(
            TownCrier.Domain.Notifications.Notification.Create(
                userId: "auth0|dormant",
                applicationName: "app-a",
                watchZoneId: "zone-1",
                applicationAddress: "10 Example St",
                applicationDescription: "Extension",
                applicationType: "HouseholderApplication",
                authorityId: 1,
                now: DateTimeOffset.UtcNow));

        // Act
        await harness.Handler.HandleAsync(new DormantAccountCleanupCommand(Now), CancellationToken.None);

        // Assert — cascade ran: notification store is empty.
        await Assert.That(harness.NotificationRepository.All).IsEmpty();
    }

    [Test]
    public async Task Should_ReturnZero_When_NoDormantProfiles()
    {
        // Arrange
        var harness = new CleanupHarness();
        await harness.Repository.SaveAsync(
            UserProfile.Register("auth0|active", email: null, now: Now.AddDays(-30)),
            CancellationToken.None);

        // Act
        var result = await harness.Handler.HandleAsync(
            new DormantAccountCleanupCommand(Now), CancellationToken.None);

        // Assert
        await Assert.That(result.DeletedCount).IsEqualTo(0);
    }

    [Test]
    public async Task Should_UseTwelveMonthCutoff_By_Default()
    {
        // Arrange — profile active exactly 12 months + 1 day ago is dormant.
        var harness = new CleanupHarness();
        await harness.Repository.SaveAsync(
            UserProfile.Register("auth0|edge-old", email: null, now: Now.AddYears(-1).AddDays(-1)),
            CancellationToken.None);

        // Profile active exactly 11 months ago is not dormant.
        await harness.Repository.SaveAsync(
            UserProfile.Register("auth0|edge-new", email: null, now: Now.AddMonths(-11)),
            CancellationToken.None);

        // Act
        var result = await harness.Handler.HandleAsync(
            new DormantAccountCleanupCommand(Now), CancellationToken.None);

        // Assert
        await Assert.That(result.DeletedCount).IsEqualTo(1);
        await Assert.That(harness.Repository.GetByUserId("auth0|edge-old")).IsNull();
        await Assert.That(harness.Repository.GetByUserId("auth0|edge-new")).IsNotNull();
    }

    private sealed class CleanupHarness
    {
        public FakeUserProfileRepository Repository { get; } = new();

        public FakeAuth0ManagementClient Auth0Client { get; } = new();

        public FakeNotificationRepository NotificationRepository { get; } = new();

        public FakeDecisionAlertRepository DecisionAlertRepository { get; } = new();

        public FakeWatchZoneRepository WatchZoneRepository { get; } = new();

        public FakeSavedApplicationRepository SavedApplicationRepository { get; } = new();

        public FakeDeviceRegistrationRepository DeviceRegistrationRepository { get; } = new();

        public DeleteUserProfileCommandHandler DeleteHandler => new(
            this.Repository,
            this.Auth0Client,
            this.NotificationRepository,
            this.DecisionAlertRepository,
            this.WatchZoneRepository,
            this.SavedApplicationRepository,
            this.DeviceRegistrationRepository);

        public DormantAccountCleanupCommandHandler Handler => new(this.Repository, this.DeleteHandler);
    }
}
