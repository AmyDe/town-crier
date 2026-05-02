using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
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

    [Test]
    public async Task Should_EmitEs256HeaderWithKid_When_Minting()
    {
        // Arrange
        var pem = ApnsJwtProviderTestKey.GeneratePkcs8Pem();
        var time = new FakeTimeProvider();
        time.SetUtcNow(new DateTimeOffset(2026, 5, 2, 12, 0, 0, TimeSpan.Zero));
        using var provider = new ApnsJwtProvider(pem, TestKeyId, TestTeamId, time);

        // Act
        var jwt = provider.Current();

        // Assert
        var header = DecodeJsonSegment(jwt.Split('.')[0]);
        await Assert.That(header.GetProperty("alg").GetString()).IsEqualTo("ES256");
        await Assert.That(header.GetProperty("kid").GetString()).IsEqualTo(TestKeyId);
    }

    [Test]
    public async Task Should_EmitIssAndIatPayload_When_Minting()
    {
        // Arrange
        var pem = ApnsJwtProviderTestKey.GeneratePkcs8Pem();
        var time = new FakeTimeProvider();
        var now = new DateTimeOffset(2026, 5, 2, 12, 0, 0, TimeSpan.Zero);
        time.SetUtcNow(now);
        using var provider = new ApnsJwtProvider(pem, TestKeyId, TestTeamId, time);

        // Act
        var jwt = provider.Current();

        // Assert
        var payload = DecodeJsonSegment(jwt.Split('.')[1]);
        await Assert.That(payload.GetProperty("iss").GetString()).IsEqualTo(TestTeamId);
        await Assert.That(payload.GetProperty("iat").GetInt64()).IsEqualTo(now.ToUnixTimeSeconds());
    }

    [Test]
    public async Task Should_SignJwtWithEs256_When_Minting()
    {
        // Arrange — use a key whose public side we keep so we can verify the signature.
        var (pem, publicVerifier) = ApnsJwtProviderTestKey.GeneratePkcs8PemWithPublicVerifier();
        using var verifier = publicVerifier;
        var time = new FakeTimeProvider();
        time.SetUtcNow(new DateTimeOffset(2026, 5, 2, 12, 0, 0, TimeSpan.Zero));
        using var provider = new ApnsJwtProvider(pem, TestKeyId, TestTeamId, time);

        // Act
        var jwt = provider.Current();

        // Assert
        var parts = jwt.Split('.');
        var signingInput = Encoding.UTF8.GetBytes($"{parts[0]}.{parts[1]}");
        var signature = DecodeBase64UrlBytes(parts[2]);
        var verified = publicVerifier.VerifyData(signingInput, signature, HashAlgorithmName.SHA256);
        await Assert.That(verified).IsTrue();
    }

    [Test]
    public async Task Should_ReturnCachedJwt_When_CalledAgainBeforeRefreshInterval()
    {
        // Arrange
        var pem = ApnsJwtProviderTestKey.GeneratePkcs8Pem();
        var time = new FakeTimeProvider();
        time.SetUtcNow(new DateTimeOffset(2026, 5, 2, 12, 0, 0, TimeSpan.Zero));
        using var provider = new ApnsJwtProvider(pem, TestKeyId, TestTeamId, time);

        // Act
        var first = provider.Current();
        time.Advance(TimeSpan.FromMinutes(49));
        var second = provider.Current();

        // Assert
        await Assert.That(second).IsEqualTo(first);
    }

    [Test]
    public async Task Should_MintFreshJwt_When_CachedTokenOlderThanRefreshInterval()
    {
        // Arrange
        var pem = ApnsJwtProviderTestKey.GeneratePkcs8Pem();
        var time = new FakeTimeProvider();
        time.SetUtcNow(new DateTimeOffset(2026, 5, 2, 12, 0, 0, TimeSpan.Zero));
        using var provider = new ApnsJwtProvider(pem, TestKeyId, TestTeamId, time);

        // Act
        var first = provider.Current();
        time.Advance(TimeSpan.FromMinutes(50) + TimeSpan.FromSeconds(1));
        var second = provider.Current();

        // Assert
        await Assert.That(second).IsNotEqualTo(first);

        // The newly minted JWT carries the advanced clock's iat.
        var payload = DecodeJsonSegment(second.Split('.')[1]);
        await Assert.That(payload.GetProperty("iat").GetInt64())
            .IsEqualTo(time.GetUtcNow().ToUnixTimeSeconds());
    }

    private static byte[] DecodeBase64UrlBytes(string base64UrlSegment)
    {
        var padded = base64UrlSegment
            .Replace('-', '+')
            .Replace('_', '/');
        switch (padded.Length % 4)
        {
            case 2: padded += "=="; break;
            case 3: padded += "="; break;
        }

        return Convert.FromBase64String(padded);
    }

    private static JsonElement DecodeJsonSegment(string base64UrlSegment)
    {
        var bytes = DecodeBase64UrlBytes(base64UrlSegment);
        var json = Encoding.UTF8.GetString(bytes);
        return JsonDocument.Parse(json).RootElement.Clone();
    }
}
