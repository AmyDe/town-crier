using TownCrier.Application.Admin;
using TownCrier.Application.Tests.UserProfiles;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.Admin;

public sealed class GrantSubscriptionCommandHandlerTests
{
    [Test]
    public async Task Should_ActivateProTier_When_UserFoundByEmail()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-1", "friend@example.com");
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new GrantSubscriptionCommandHandler(repository);
        var command = new GrantSubscriptionCommand("friend@example.com", SubscriptionTier.Pro);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Pro);
        await Assert.That(result.Email).IsEqualTo("friend@example.com");
    }

    [Test]
    public async Task Should_ActivatePersonalTier_When_Requested()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-1", "friend@example.com");
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new GrantSubscriptionCommandHandler(repository);
        var command = new GrantSubscriptionCommand("friend@example.com", SubscriptionTier.Personal);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Personal);
    }

    [Test]
    public async Task Should_RevokeToFree_When_FreeTierRequested()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-1", "friend@example.com");
        profile.ActivateSubscription(SubscriptionTier.Pro, DateTimeOffset.UtcNow.AddYears(73));
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new GrantSubscriptionCommandHandler(repository);
        var command = new GrantSubscriptionCommand("friend@example.com", SubscriptionTier.Free);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Free);
    }

    [Test]
    public async Task Should_PersistTierChange_When_Granted()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-1", "friend@example.com");
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new GrantSubscriptionCommandHandler(repository);
        var command = new GrantSubscriptionCommand("friend@example.com", SubscriptionTier.Pro);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var saved = repository.GetByUserId("auth0|user-1");
        await Assert.That(saved!.Tier).IsEqualTo(SubscriptionTier.Pro);
    }

    [Test]
    public async Task Should_ThrowUserProfileNotFoundException_When_EmailNotFound()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var handler = new GrantSubscriptionCommandHandler(repository);
        var command = new GrantSubscriptionCommand("nobody@example.com", SubscriptionTier.Pro);

        // Act & Assert
        await Assert.ThrowsAsync<UserProfileNotFoundException>(
            () => handler.HandleAsync(command, CancellationToken.None));
    }
}
