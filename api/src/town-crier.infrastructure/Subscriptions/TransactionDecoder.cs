using System.Text.Json;
using TownCrier.Application.Subscriptions;

namespace TownCrier.Infrastructure.Subscriptions;

/// <summary>
/// Maps the raw JSON payload of a verified Apple JWS transaction onto the
/// application-layer <see cref="DecodedTransaction"/>. Native AOT-safe — uses
/// System.Text.Json source generation only.
/// </summary>
public sealed class TransactionDecoder : ITransactionDecoder
{
    public DecodedTransaction Decode(string json)
    {
        if (string.IsNullOrWhiteSpace(json))
        {
            throw new ArgumentException("The transaction JSON is empty.", nameof(json));
        }

        AppleTransactionPayload? payload;
        try
        {
            payload = JsonSerializer.Deserialize(
                json, SubscriptionsJsonSerializerContext.Default.AppleTransactionPayload);
        }
        catch (JsonException ex)
        {
            throw new ArgumentException("The transaction JSON is malformed.", nameof(json), ex);
        }

        if (payload is null)
        {
            throw new ArgumentException("The transaction JSON is null.", nameof(json));
        }

        return new DecodedTransaction(
            TransactionId: Require(payload.TransactionId, "transactionId"),
            OriginalTransactionId: Require(payload.OriginalTransactionId, "originalTransactionId"),
            ProductId: Require(payload.ProductId, "productId"),
            BundleId: Require(payload.BundleId, "bundleId"),
            PurchaseDate: DateTimeOffset.FromUnixTimeMilliseconds(payload.PurchaseDate),
            ExpiresDate: DateTimeOffset.FromUnixTimeMilliseconds(payload.ExpiresDate),
            Environment: Require(payload.Environment, "environment"));
    }

    private static string Require(string? value, string field) =>
        string.IsNullOrEmpty(value)
            ? throw new ArgumentException(
                $"The transaction JSON is missing the required '{field}' field.")
            : value;
}
