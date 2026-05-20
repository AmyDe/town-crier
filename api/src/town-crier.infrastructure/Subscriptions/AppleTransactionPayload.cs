using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Subscriptions;

/// <summary>
/// The decoded JSON payload of an Apple JWS transaction (the
/// <c>JWSTransactionDecodedPayload</c> shape from StoreKit 2). Dates are Unix
/// epoch milliseconds, per Apple's encoding.
/// </summary>
internal sealed record AppleTransactionPayload(
    [property: JsonPropertyName("transactionId")] string? TransactionId,
    [property: JsonPropertyName("originalTransactionId")] string? OriginalTransactionId,
    [property: JsonPropertyName("productId")] string? ProductId,
    [property: JsonPropertyName("bundleId")] string? BundleId,
    [property: JsonPropertyName("purchaseDate")] long PurchaseDate,
    [property: JsonPropertyName("expiresDate")] long ExpiresDate,
    [property: JsonPropertyName("environment")] string? Environment);
