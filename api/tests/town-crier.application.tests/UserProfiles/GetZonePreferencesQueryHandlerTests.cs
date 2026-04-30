using TownCrier.Application.Tests.Notifications;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.UserProfiles;

public sealed class GetZonePreferencesQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnDefaultPreferences_When_NoneSet()
    {
        // Arrange
        var (handler, repo) = CreateHandler();
        var profile = new UserProfileBuilder().WithUserId("user-1").Build();
        await repo.SaveAsync(profile, CancellationToken.None);

        // Act
        var result = await handler.HandleAsync(
            new GetZonePreferencesQuery("user-1", "zone-1"),
            CancellationToken.None);

        // Assert
        await Assert.That(result.ZoneId).IsEqualTo("zone-1");
        await Assert.That(result.NewApplicationPush).IsTrue();
        await Assert.That(result.NewApplicationEmail).IsTrue();
        await Assert.That(result.DecisionPush).IsTrue();
        await Assert.That(result.DecisionEmail).IsTrue();
    }

    [Test]
    public async Task Should_ReturnSavedPreferences_When_PreviouslySet()
    {
        // Arrange
        var (handler, repo) = CreateHandler();
        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Pro)
            .Build();
        profile.SetZonePreferences(
            "zone-1",
            new ZoneNotificationPreferences(
                NewApplicationPush: true,
                NewApplicationEmail: true,
                DecisionPush: false,
                DecisionEmail: false));
        await repo.SaveAsync(profile, CancellationToken.None);

        // Act
        var result = await handler.HandleAsync(
            new GetZonePreferencesQuery("user-1", "zone-1"),
            CancellationToken.None);

        // Assert
        await Assert.That(result.NewApplicationPush).IsTrue();
        await Assert.That(result.NewApplicationEmail).IsTrue();
        await Assert.That(result.DecisionPush).IsFalse();
        await Assert.That(result.DecisionEmail).IsFalse();
    }

    [Test]
    public async Task Should_ThrowNotFound_When_UserDoesNotExist()
    {
        // Arrange
        var (handler, _) = CreateHandler();

        // Act & Assert
        await Assert.ThrowsAsync<UserProfileNotFoundException>(
            () => handler.HandleAsync(
                new GetZonePreferencesQuery("nonexistent", "zone-1"),
                CancellationToken.None));
    }

    private static (GetZonePreferencesQueryHandler Handler, FakeUserProfileRepository Repo) CreateHandler()
    {
        var repo = new FakeUserProfileRepository();
        var handler = new GetZonePreferencesQueryHandler(repo);
        return (handler, repo);
    }
}
