using TownCrier.Application.Subscriptions;

namespace TownCrier.Web.Tests.Subscriptions;

/// <summary>
/// Test double for <see cref="IAppleJwsVerifier"/> that maps each signed JWS
/// string to a fixed decoded payload. The App Store webhook verifies two
/// nested JWS payloads (outer notification, inner transaction), so the stub
/// must distinguish them by input. Configure with <see cref="WithPayload"/>,
/// or use <see cref="ThatRejects"/> to simulate an invalid signature.
/// </summary>
internal sealed class MappingAppleJwsVerifier : IAppleJwsVerifier
{
    private readonly Dictionary<string, string> payloads = [];
    private readonly bool rejects;

    private MappingAppleJwsVerifier(bool rejects) => this.rejects = rejects;

    public static MappingAppleJwsVerifier Create() => new(rejects: false);

    public static MappingAppleJwsVerifier ThatRejects() => new(rejects: true);

    public MappingAppleJwsVerifier WithPayload(string signedJws, string decodedJson)
    {
        this.payloads[signedJws] = decodedJson;
        return this;
    }

    public Task<string> VerifyAndDecodeAsync(string signedPayload, CancellationToken ct)
    {
        if (this.rejects)
        {
            throw new AppleJwsVerificationException("Invalid JWS signature.");
        }

        if (this.payloads.TryGetValue(signedPayload, out var json))
        {
            return Task.FromResult(json);
        }

        throw new AppleJwsVerificationException($"Unknown signed payload: '{signedPayload}'.");
    }
}
