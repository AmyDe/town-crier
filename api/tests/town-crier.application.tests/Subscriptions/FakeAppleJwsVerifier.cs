using TownCrier.Application.Subscriptions;

namespace TownCrier.Application.Tests.Subscriptions;

internal sealed class FakeAppleJwsVerifier : IAppleJwsVerifier
{
    private readonly Dictionary<string, string> payloads = [];
    private bool shouldFail;

    public void SetPayload(string signedPayload, string decodedJson)
    {
        this.payloads[signedPayload] = decodedJson;
    }

    public void SetShouldFail()
    {
        this.shouldFail = true;
    }

    public Task<string> VerifyAndDecodeAsync(string signedPayload, CancellationToken ct)
    {
        if (this.shouldFail)
        {
            throw new AppleJwsVerificationException("JWS verification failed.");
        }

        if (this.payloads.TryGetValue(signedPayload, out var json))
        {
            return Task.FromResult(json);
        }

        throw new AppleJwsVerificationException($"Unknown signed payload: '{signedPayload}'");
    }
}
