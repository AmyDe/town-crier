using TownCrier.Application.Tests.Admin;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.UserProfiles;

public sealed class CreateUserProfileCommandHandlerTests
{
    [Test]
    public async Task Should_CreateProfileWithFreeTier_When_NewUser()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var handler = new CreateUserProfileCommandHandler(repository, NoAutoGrant(), new FakeAuth0ManagementClient());
        var command = new CreateUserProfileCommand("auth0|user-123");

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.UserId).IsEqualTo("auth0|user-123");
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Free);
        await Assert.That(result.PushEnabled).IsTrue();
    }

    [Test]
    public async Task Should_PersistProfile_When_NewUser()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var handler = new CreateUserProfileCommandHandler(repository, NoAutoGrant(), new FakeAuth0ManagementClient());
        var command = new CreateUserProfileCommand("auth0|user-123");

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var saved = repository.GetByUserId("auth0|user-123");
        await Assert.That(saved).IsNotNull();
        await Assert.That(saved!.Tier).IsEqualTo(SubscriptionTier.Free);
    }

    [Test]
    public async Task Should_StoreEmail_When_ProfileCreated()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var handler = new CreateUserProfileCommandHandler(repository, NoAutoGrant(), new FakeAuth0ManagementClient());
        var command = new CreateUserProfileCommand("auth0|user-email", "user@example.com");

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var saved = repository.GetByUserId("auth0|user-email");
        await Assert.That(saved!.Email).IsEqualTo("user@example.com");
    }

    [Test]
    public async Task Should_ReturnExistingProfile_When_UserAlreadyExists()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var handler = new CreateUserProfileCommandHandler(repository, NoAutoGrant(), new FakeAuth0ManagementClient());
        var command = new CreateUserProfileCommand("auth0|user-123");

        // Create first
        await handler.HandleAsync(command, CancellationToken.None);

        // Act — create again
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert — still one profile, returns existing
        await Assert.That(repository.Count).IsEqualTo(1);
        await Assert.That(result.UserId).IsEqualTo("auth0|user-123");
    }

    [Test]
    public async Task Should_AutoGrantProTier_When_EmailMatchesConfiguredDomain()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var handler = new CreateUserProfileCommandHandler(repository, AutoGrantFor("family.uk"), new FakeAuth0ManagementClient());
        var command = new CreateUserProfileCommand("auth0|family-1", "alice@family.uk", EmailVerified: true);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Pro);
        var saved = repository.GetByUserId("auth0|family-1");
        await Assert.That(saved!.Tier).IsEqualTo(SubscriptionTier.Pro);
    }

    [Test]
    public async Task Should_RemainFreeTier_When_EmailNotVerified()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var handler = new CreateUserProfileCommandHandler(repository, AutoGrantFor("family.uk"), new FakeAuth0ManagementClient());
        var command = new CreateUserProfileCommand("auth0|unverified-1", "alice@family.uk", EmailVerified: false);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Free);
    }

    [Test]
    public async Task Should_RemainFreeTier_When_EmailDoesNotMatchConfiguredDomain()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var handler = new CreateUserProfileCommandHandler(repository, AutoGrantFor("family.uk"), new FakeAuth0ManagementClient());
        var command = new CreateUserProfileCommand("auth0|other-1", "someone@gmail.com", EmailVerified: true);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Free);
    }

    [Test]
    public async Task Should_RemainFreeTier_When_NoEmailProvided()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var handler = new CreateUserProfileCommandHandler(repository, AutoGrantFor("family.uk"), new FakeAuth0ManagementClient());
        var command = new CreateUserProfileCommand("auth0|no-email");

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Free);
    }

    [Test]
    public async Task Should_AutoGrantProTier_When_EmailMatchesAnyConfiguredDomain()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var handler = new CreateUserProfileCommandHandler(repository, AutoGrantFor("family.uk, vip.com"), new FakeAuth0ManagementClient());
        var command = new CreateUserProfileCommand("auth0|vip-1", "bob@vip.com", EmailVerified: true);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Pro);
    }

    [Test]
    public async Task Should_MatchDomainCaseInsensitively()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var handler = new CreateUserProfileCommandHandler(repository, AutoGrantFor("family.uk"), new FakeAuth0ManagementClient());
        var command = new CreateUserProfileCommand("auth0|upper-1", "Alice@FAMILY.UK", EmailVerified: true);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Pro);
    }

    [Test]
    public async Task Should_NotAutoGrant_When_ExistingUserReturned()
    {
        // Arrange — existing user is on Free tier
        var repository = new FakeUserProfileRepository();
        var noAutoGrant = new CreateUserProfileCommandHandler(repository, NoAutoGrant(), new FakeAuth0ManagementClient());
        await noAutoGrant.HandleAsync(
            new CreateUserProfileCommand("auth0|existing", "alice@family.uk", EmailVerified: true), CancellationToken.None);

        // Act — re-register with auto-grant enabled (should return existing, not upgrade)
        var handler = new CreateUserProfileCommandHandler(repository, AutoGrantFor("family.uk"), new FakeAuth0ManagementClient());
        var result = await handler.HandleAsync(
            new CreateUserProfileCommand("auth0|existing", "alice@family.uk", EmailVerified: true), CancellationToken.None);

        // Assert — still Free because existing profile is returned as-is
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Free);
    }

    [Test]
    public async Task Should_BackfillEmail_When_ExistingProfileHasNullEmail()
    {
        // Arrange — profile created without email (the bug scenario)
        var repository = new FakeUserProfileRepository();
        var handler = new CreateUserProfileCommandHandler(repository, NoAutoGrant(), new FakeAuth0ManagementClient());
        await handler.HandleAsync(
            new CreateUserProfileCommand("auth0|no-email-user"), CancellationToken.None);

        var before = repository.GetByUserId("auth0|no-email-user");
        await Assert.That(before!.Email).IsNull();

        // Act — re-register now that the token includes email
        await handler.HandleAsync(
            new CreateUserProfileCommand("auth0|no-email-user", "user@example.com"), CancellationToken.None);

        // Assert — email has been backfilled
        var after = repository.GetByUserId("auth0|no-email-user");
        await Assert.That(after!.Email).IsEqualTo("user@example.com");
    }

    [Test]
    public async Task Should_NotOverwriteEmail_When_ExistingProfileAlreadyHasEmail()
    {
        // Arrange — profile created with email
        var repository = new FakeUserProfileRepository();
        var handler = new CreateUserProfileCommandHandler(repository, NoAutoGrant(), new FakeAuth0ManagementClient());
        await handler.HandleAsync(
            new CreateUserProfileCommand("auth0|has-email", "original@example.com"), CancellationToken.None);

        // Act — re-register with a different email
        await handler.HandleAsync(
            new CreateUserProfileCommand("auth0|has-email", "different@example.com"), CancellationToken.None);

        // Assert — original email preserved
        var saved = repository.GetByUserId("auth0|has-email");
        await Assert.That(saved!.Email).IsEqualTo("original@example.com");
    }

    private static AutoGrantOptions NoAutoGrant() => new();

    private static AutoGrantOptions AutoGrantFor(string domains) => new() { ProDomains = domains };
}
