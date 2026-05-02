using TownCrier.Infrastructure.Notifications;
using TownCrier.Infrastructure.Tests.Polling;

namespace TownCrier.Infrastructure.Tests.Notifications;

public sealed class ApnsJwtProviderTests
{
    private const string TestKeyId = "ABC1234567";
    private const string TestTeamId = "DEF7654321";

    [Test]
    public async Task Should_MintNonEmptyJwt_When_CurrentCalled()
    {
        // Arrange
        var pem = ApnsJwtProviderTestKey.GeneratePkcs8Pem();
        var time = new FakeTimeProvider();
        time.SetUtcNow(new DateTimeOffset(2026, 5, 2, 12, 0, 0, TimeSpan.Zero));
        using var provider = new ApnsJwtProvider(pem, TestKeyId, TestTeamId, time);

        // Act
        var jwt = provider.Current();

        // Assert
        await Assert.That(jwt).IsNotNull();
        await Assert.That(jwt.Length).IsGreaterThan(0);

        // A JWT has three base64url segments separated by dots.
        await Assert.That(jwt.Split('.').Length).IsEqualTo(3);
    }
}
