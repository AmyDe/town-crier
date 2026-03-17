using TownCrier.Application.Tests.Notifications;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.UserProfiles;

public sealed class UpdateZonePreferencesCommandHandlerTests
{
    [Test]
    public async Task Should_SetZonePreferences_When_FreeTierEnablesNewApplications()
    {
        // Arrange
        var (handler, repo) = CreateHandler();
        var profile = new UserProfileBuilder().WithUserId("user-1").Build();
        await repo.SaveAsync(profile, CancellationToken.None);

        var command = new UpdateZonePreferencesCommand(
            "user-1",
            "zone-1",
            NewApplications: true,
            StatusChanges: false,
            DecisionUpdates: false);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.NewApplications).IsTrue();
        await Assert.That(result.StatusChanges).IsFalse();
        await Assert.That(result.DecisionUpdates).IsFalse();
    }

    [Test]
    public async Task Should_RejectStatusChanges_When_FreeTier()
    {
        // Arrange
        var (handler, repo) = CreateHandler();
        var profile = new UserProfileBuilder().WithUserId("user-1").Build();
        await repo.SaveAsync(profile, CancellationToken.None);

        var command = new UpdateZonePreferencesCommand(
            "user-1",
            "zone-1",
            NewApplications: true,
            StatusChanges: true,
            DecisionUpdates: false);

        // Act & Assert
        await Assert.ThrowsAsync<InsufficientTierException>(
            () => handler.HandleAsync(command, CancellationToken.None));
    }

    [Test]
    public async Task Should_RejectDecisionUpdates_When_FreeTier()
    {
        // Arrange
        var (handler, repo) = CreateHandler();
        var profile = new UserProfileBuilder().WithUserId("user-1").Build();
        await repo.SaveAsync(profile, CancellationToken.None);

        var command = new UpdateZonePreferencesCommand(
            "user-1",
            "zone-1",
            NewApplications: true,
            StatusChanges: false,
            DecisionUpdates: true);

        // Act & Assert
        await Assert.ThrowsAsync<InsufficientTierException>(
            () => handler.HandleAsync(command, CancellationToken.None));
    }

    [Test]
    public async Task Should_AllowAllPreferences_When_ProTier()
    {
        // Arrange
        var (handler, repo) = CreateHandler();
        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Pro)
            .Build();
        await repo.SaveAsync(profile, CancellationToken.None);

        var command = new UpdateZonePreferencesCommand(
            "user-1",
            "zone-1",
            NewApplications: true,
            StatusChanges: true,
            DecisionUpdates: true);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.NewApplications).IsTrue();
        await Assert.That(result.StatusChanges).IsTrue();
        await Assert.That(result.DecisionUpdates).IsTrue();
    }

    [Test]
    public async Task Should_PersistPreferences_When_Updated()
    {
        // Arrange
        var (handler, repo) = CreateHandler();
        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Pro)
            .Build();
        await repo.SaveAsync(profile, CancellationToken.None);

        var command = new UpdateZonePreferencesCommand(
            "user-1",
            "zone-1",
            NewApplications: true,
            StatusChanges: true,
            DecisionUpdates: false);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var saved = await repo.GetByUserIdAsync("user-1", CancellationToken.None);
        var zonePrefs = saved!.GetZonePreferences("zone-1");
        await Assert.That(zonePrefs.NewApplications).IsTrue();
        await Assert.That(zonePrefs.StatusChanges).IsTrue();
        await Assert.That(zonePrefs.DecisionUpdates).IsFalse();
    }

    [Test]
    public async Task Should_ThrowNotFound_When_UserDoesNotExist()
    {
        // Arrange
        var (handler, _) = CreateHandler();

        var command = new UpdateZonePreferencesCommand(
            "nonexistent",
            "zone-1",
            NewApplications: true,
            StatusChanges: false,
            DecisionUpdates: false);

        // Act & Assert
        await Assert.ThrowsAsync<UserProfileNotFoundException>(
            () => handler.HandleAsync(command, CancellationToken.None));
    }

    [Test]
    public async Task Should_ReturnDefaults_When_ZoneHasNoPreferences()
    {
        // Arrange
        var profile = new UserProfileBuilder().WithUserId("user-1").Build();

        // Act — zone with no explicit preferences gets defaults
        var zonePrefs = profile.GetZonePreferences("zone-99");

        // Assert
        await Assert.That(zonePrefs.NewApplications).IsTrue();
        await Assert.That(zonePrefs.StatusChanges).IsFalse();
        await Assert.That(zonePrefs.DecisionUpdates).IsFalse();
    }

    [Test]
    public async Task Should_DisableNewApplications_When_ExplicitlyTurnedOff()
    {
        // Arrange
        var (handler, repo) = CreateHandler();
        var profile = new UserProfileBuilder().WithUserId("user-1").Build();
        await repo.SaveAsync(profile, CancellationToken.None);

        var command = new UpdateZonePreferencesCommand(
            "user-1",
            "zone-1",
            NewApplications: false,
            StatusChanges: false,
            DecisionUpdates: false);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.NewApplications).IsFalse();
    }

    private static (UpdateZonePreferencesCommandHandler Handler, FakeUserProfileRepository Repo) CreateHandler()
    {
        var repo = new FakeUserProfileRepository();
        var handler = new UpdateZonePreferencesCommandHandler(repo);
        return (handler, repo);
    }
}
