using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Subscriptions;

/// <summary>
/// The <c>data</c> object of an App Store Server Notification v2, carrying the
/// nested signed JWS payloads.
/// </summary>
internal sealed record AppleNotificationData(
    [property: JsonPropertyName("signedTransactionInfo")] string? SignedTransactionInfo,
    [property: JsonPropertyName("signedRenewalInfo")] string? SignedRenewalInfo);
