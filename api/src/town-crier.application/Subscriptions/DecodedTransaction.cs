namespace TownCrier.Application.Subscriptions;

/// <summary>
/// Represents the decoded payload from an Apple JWS transaction.
/// This is an application-layer DTO; the infrastructure adapter maps from
/// the raw JSON to this type.
/// </summary>
public sealed record DecodedTransaction(
    string TransactionId,
    string OriginalTransactionId,
    string ProductId,
    string BundleId,
    DateTimeOffset PurchaseDate,
    DateTimeOffset ExpiresDate,
    string Environment);
