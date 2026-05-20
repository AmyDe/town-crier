using TownCrier.Application.Subscriptions;

namespace TownCrier.Web.Tests.Subscriptions;

/// <summary>
/// Test double for <see cref="IAppleJwsVerifier"/>. Returns a fixed payload
/// for the configured signed transaction, or throws to simulate an invalid
/// signature.
/// </summary>
internal sealed class StubAppleJwsVerifier : IAppleJwsVerifier
{
    private readonly string? decodedJson;

    private StubAppleJwsVerifier(string? decodedJson)
    {
        this.decodedJson = decodedJson;
    }

    public static StubAppleJwsVerifier ReturningPayload(string decodedJson) => new(decodedJson);

    public static StubAppleJwsVerifier ThatRejects() => new(decodedJson: null);

    public Task<string> VerifyAndDecodeAsync(string signedPayload, CancellationToken ct)
    {
        if (this.decodedJson is null)
        {
            throw new AppleJwsVerificationException("Invalid JWS signature.");
        }

        return Task.FromResult(this.decodedJson);
    }
}
