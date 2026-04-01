using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.UserProfiles;

public sealed class FakeUserProfileRepositoryTests
{
    [Test]
    public async Task GetByEmailAsync_Should_ReturnProfile_When_EmailMatches()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-1", "test@example.com");
        await repository.SaveAsync(profile, CancellationToken.None);

        // Act
        var result = await repository.GetByEmailAsync("test@example.com", CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.UserId).IsEqualTo("auth0|user-1");
    }

    [Test]
    public async Task GetByEmailAsync_Should_ReturnNull_When_NoMatch()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();

        // Act
        var result = await repository.GetByEmailAsync("nobody@example.com", CancellationToken.None);

        // Assert
        await Assert.That(result).IsNull();
    }
}
