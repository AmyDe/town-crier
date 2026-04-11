namespace TownCrier.Application.Subscriptions;

public interface IAppleJwsVerifier
{
    /// <summary>
    /// Verifies an Apple JWS compact serialization and returns the decoded JSON payload.
    /// Throws <see cref="AppleJwsVerificationException"/> if verification fails.
    /// </summary>
    Task<string> VerifyAndDecodeAsync(string signedPayload, CancellationToken ct);
}
