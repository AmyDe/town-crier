namespace TownCrier.Application.Subscriptions;

public interface IAppleJwsVerifier
{
    /// <summary>
    /// Verifies an Apple JWS compact serialization and returns the decoded JSON payload.
    /// Throws <see cref="AppleJwsVerificationException"/> if verification fails.
    /// </summary>
    /// <param name="signedPayload">The JWS compact serialization to verify.</param>
    /// <param name="ct">Cancellation token.</param>
    /// <returns>The decoded JSON payload string.</returns>
    Task<string> VerifyAndDecodeAsync(string signedPayload, CancellationToken ct);
}
