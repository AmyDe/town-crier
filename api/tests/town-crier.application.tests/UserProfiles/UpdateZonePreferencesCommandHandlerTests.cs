using TownCrier.Application.Tests.Notifications;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.UserProfiles;

public sealed class UpdateZonePreferencesCommandHandlerTests
{
    [Test]
    public async Task Should_SetZonePreferences_When_FreeTierEnablesNewApplicationPush()
    {
        // Arrange
        var (handler, repo) = CreateHandler();
        var profile = new UserProfileBuilder().WithUserId("user-1").Build();
        await repo.SaveAsync(profile, CancellationToken.None);

        var command = new UpdateZonePreferencesCommand(
            "user-1",
            "zone-1",
            NewApplicationPush: true,
            NewApplicationEmail: false,
            DecisionPush: false,
            DecisionEmail: false);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.NewApplicationPush).IsTrue();
        await Assert.That(result.NewApplicationEmail).IsFalse();
        await Assert.That(result.DecisionPush).IsFalse();
        await Assert.That(result.DecisionEmail).IsFalse();
    }

    [Test]
    public async Task Should_AllowAllPreferences_When_FreeTier()
    {
        // Arrange — free-tier guard removed: domain no longer blocks any toggle.
        var (handler, repo) = CreateHandler();
        var profile = new UserProfileBuilder().WithUserId("user-1").Build();
        await repo.SaveAsync(profile, CancellationToken.None);

        var command = new UpdateZonePreferencesCommand(
            "user-1",
            "zone-1",
            NewApplicationPush: true,
            NewApplicationEmail: true,
            DecisionPush: true,
            DecisionEmail: true);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.NewApplicationPush).IsTrue();
        await Assert.That(result.NewApplicationEmail).IsTrue();
        await Assert.That(result.DecisionPush).IsTrue();
        await Assert.That(result.DecisionEmail).IsTrue();
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
            NewApplicationPush: true,
            NewApplicationEmail: true,
            DecisionPush: true,
            DecisionEmail: true);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.NewApplicationPush).IsTrue();
        await Assert.That(result.NewApplicationEmail).IsTrue();
        await Assert.That(result.DecisionPush).IsTrue();
        await Assert.That(result.DecisionEmail).IsTrue();
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
            NewApplicationPush: true,
            NewApplicationEmail: true,
            DecisionPush: true,
            DecisionEmail: false);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var saved = await repo.GetByUserIdAsync("user-1", CancellationToken.None);
        var zonePrefs = saved!.GetZonePreferences("zone-1");
        await Assert.That(zonePrefs.NewApplicationPush).IsTrue();
        await Assert.That(zonePrefs.NewApplicationEmail).IsTrue();
        await Assert.That(zonePrefs.DecisionPush).IsTrue();
        await Assert.That(zonePrefs.DecisionEmail).IsFalse();
    }

    [Test]
    public async Task Should_ThrowNotFound_When_UserDoesNotExist()
    {
        // Arrange
        var (handler, _) = CreateHandler();

        var command = new UpdateZonePreferencesCommand(
            "nonexistent",
            "zone-1",
            NewApplicationPush: true,
            NewApplicationEmail: true,
            DecisionPush: true,
            DecisionEmail: true);

        // Act & Assert
        await Assert.ThrowsAsync<UserProfileNotFoundException>(
            () => handler.HandleAsync(command, CancellationToken.None));
    }

    [Test]
    public async Task Should_ReturnDefaults_When_ZoneHasNoPreferences()
    {
        // Arrange
        var profile = new UserProfileBuilder().WithUserId("user-1").Build();

        // Act — zone with no explicit preferences gets defaults (all true)
        var zonePrefs = profile.GetZonePreferences("zone-99");

        // Assert
        await Assert.That(zonePrefs.NewApplicationPush).IsTrue();
        await Assert.That(zonePrefs.NewApplicationEmail).IsTrue();
        await Assert.That(zonePrefs.DecisionPush).IsTrue();
        await Assert.That(zonePrefs.DecisionEmail).IsTrue();
    }

    [Test]
    public async Task Should_DisableNewApplicationPush_When_ExplicitlyTurnedOff()
    {
        // Arrange
        var (handler, repo) = CreateHandler();
        var profile = new UserProfileBuilder().WithUserId("user-1").Build();
        await repo.SaveAsync(profile, CancellationToken.None);

        var command = new UpdateZonePreferencesCommand(
            "user-1",
            "zone-1",
            NewApplicationPush: false,
            NewApplicationEmail: false,
            DecisionPush: false,
            DecisionEmail: false);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.NewApplicationPush).IsFalse();
    }

    private static (UpdateZonePreferencesCommandHandler Handler, FakeUserProfileRepository Repo) CreateHandler()
    {
        var repo = new FakeUserProfileRepository();
        var handler = new UpdateZonePreferencesCommandHandler(repo);
        return (handler, repo);
    }
}
