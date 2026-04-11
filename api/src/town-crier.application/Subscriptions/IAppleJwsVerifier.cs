namespace TownCrier.Application.Subscriptions;

public interface IAppleJwsVerifier
{
    /// <summary>
    /// Verifies an Apple JWS compact serialization and returns the decoded JSON payload.
    /// Throws <see cref="AppleJwsVerificationException"/> if verification fails.
    /// </summary>
    /// <returns><placeholder>A <see cref="Task"/> representing the asynchronous operation.</placeholder></returns>
    Task<string> VerifyAndDecodeAsync(string signedPayload, CancellationToken ct);
}
