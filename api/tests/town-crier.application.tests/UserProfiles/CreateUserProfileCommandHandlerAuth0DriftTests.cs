using TownCrier.Application.Auth;
using TownCrier.Application.Tests.Admin;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.UserProfiles;

public sealed class CreateUserProfileCommandHandlerAuth0DriftTests
{
    [Test]
    public async Task Should_NotCallAuth0_When_ExistingProfileTierMatchesJwtClaim()
    {
        // Arrange — existing Pro profile, JWT also says Pro
        var repository = new FakeUserProfileRepository();
        var auth0 = new FakeAuth0ManagementClient();
        var seedHandler = new CreateUserProfileCommandHandler(repository, NoAutoGrant(), auth0);
        await seedHandler.HandleAsync(
            new CreateUserProfileCommand("auth0|match", "user@example.com", EmailVerified: true), CancellationToken.None);

        var existing = repository.GetByUserId("auth0|match")!;
        existing.ActivateSubscription(SubscriptionTier.Pro, new DateTimeOffset(2099, 12, 31, 0, 0, 0, TimeSpan.Zero));
        await repository.SaveAsync(existing, CancellationToken.None);

        auth0.Updates.Clear(); // discard any seed-time calls (there should be none)

        var handler = new CreateUserProfileCommandHandler(repository, NoAutoGrant(), auth0);
        var command = new CreateUserProfileCommand(
            "auth0|match", "user@example.com", EmailVerified: true, JwtSubscriptionTier: "Pro");

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(auth0.Updates).IsEmpty();
    }

    [Test]
    public async Task Should_CallAuth0UpdateSubscriptionTier_When_ExistingProfileTierIsProButJwtClaimIsFree()
    {
        // Arrange — existing Pro profile, JWT incorrectly says Free (the bug scenario)
        var repository = new FakeUserProfileRepository();
        var auth0 = new FakeAuth0ManagementClient();
        var seedHandler = new CreateUserProfileCommandHandler(repository, NoAutoGrant(), auth0);
        await seedHandler.HandleAsync(
            new CreateUserProfileCommand("auth0|drift", "user@example.com", EmailVerified: true), CancellationToken.None);

        var existing = repository.GetByUserId("auth0|drift")!;
        existing.ActivateSubscription(SubscriptionTier.Pro, new DateTimeOffset(2099, 12, 31, 0, 0, 0, TimeSpan.Zero));
        await repository.SaveAsync(existing, CancellationToken.None);

        auth0.Updates.Clear();

        var handler = new CreateUserProfileCommandHandler(repository, NoAutoGrant(), auth0);
        var command = new CreateUserProfileCommand(
            "auth0|drift", "user@example.com", EmailVerified: true, JwtSubscriptionTier: "Free");

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert — drift detected: backfill Auth0 with the Cosmos-truth Pro tier
        await Assert.That(auth0.Updates).HasCount().EqualTo(1);
        await Assert.That(auth0.Updates[0].UserId).IsEqualTo("auth0|drift");
        await Assert.That(auth0.Updates[0].Tier).IsEqualTo("Pro");
    }

    [Test]
    public async Task Should_StillReturnResult_When_Auth0UpdateThrows()
    {
        // Arrange — existing Pro profile, JWT says Free, Auth0 client will throw
        var repository = new FakeUserProfileRepository();
        var auth0 = new ThrowingAuth0ManagementClient();
        var seedHandler = new CreateUserProfileCommandHandler(repository, NoAutoGrant(), new FakeAuth0ManagementClient());
        await seedHandler.HandleAsync(
            new CreateUserProfileCommand("auth0|throws", "user@example.com", EmailVerified: true), CancellationToken.None);

        var existing = repository.GetByUserId("auth0|throws")!;
        existing.ActivateSubscription(SubscriptionTier.Pro, new DateTimeOffset(2099, 12, 31, 0, 0, 0, TimeSpan.Zero));
        await repository.SaveAsync(existing, CancellationToken.None);

        var handler = new CreateUserProfileCommandHandler(repository, NoAutoGrant(), auth0);
        var command = new CreateUserProfileCommand(
            "auth0|throws", "user@example.com", EmailVerified: true, JwtSubscriptionTier: "Free");

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert — handler swallows the Auth0 failure and still returns the existing profile
        await Assert.That(result.UserId).IsEqualTo("auth0|throws");
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Pro);
    }

    [Test]
    public async Task Should_NotCallAuth0_When_JwtSubscriptionTierClaimIsMissing()
    {
        // Arrange — existing Pro profile, no JWT tier claim available (token issued before claim was added)
        var repository = new FakeUserProfileRepository();
        var auth0 = new FakeAuth0ManagementClient();
        var seedHandler = new CreateUserProfileCommandHandler(repository, NoAutoGrant(), auth0);
        await seedHandler.HandleAsync(
            new CreateUserProfileCommand("auth0|noclaim", "user@example.com", EmailVerified: true), CancellationToken.None);

        var existing = repository.GetByUserId("auth0|noclaim")!;
        existing.ActivateSubscription(SubscriptionTier.Pro, new DateTimeOffset(2099, 12, 31, 0, 0, 0, TimeSpan.Zero));
        await repository.SaveAsync(existing, CancellationToken.None);

        auth0.Updates.Clear();

        var handler = new CreateUserProfileCommandHandler(repository, NoAutoGrant(), auth0);
        var command = new CreateUserProfileCommand(
            "auth0|noclaim", "user@example.com", EmailVerified: true, JwtSubscriptionTier: null);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert — no claim means no drift signal; do not call Auth0
        await Assert.That(auth0.Updates).IsEmpty();
    }

    [Test]
    public async Task Should_NotCallAuth0_When_NewProfileIsCreated()
    {
        // Arrange — first-ever POST /v1/me; nothing in Cosmos yet, JWT claim says Free
        var repository = new FakeUserProfileRepository();
        var auth0 = new FakeAuth0ManagementClient();
        var handler = new CreateUserProfileCommandHandler(repository, NoAutoGrant(), auth0);
        var command = new CreateUserProfileCommand(
            "auth0|new", "user@example.com", EmailVerified: true, JwtSubscriptionTier: "Free");

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert — no existing profile means no drift to backfill
        await Assert.That(auth0.Updates).IsEmpty();
    }

    private static AutoGrantOptions NoAutoGrant() => new();

    private sealed class ThrowingAuth0ManagementClient : IAuth0ManagementClient
    {
        public Task UpdateSubscriptionTierAsync(string userId, string tier, CancellationToken ct)
            => throw new InvalidOperationException("Auth0 management API is unreachable.");

        public Task DeleteUserAsync(string userId, CancellationToken ct)
            => throw new InvalidOperationException("Auth0 management API is unreachable.");
    }
}
