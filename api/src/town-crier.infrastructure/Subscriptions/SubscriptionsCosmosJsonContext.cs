using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Subscriptions;

/// <summary>
/// Source-generated JSON metadata for the subscription Cosmos documents — keeps
/// the idempotency store Native AOT-compatible (no reflection-based
/// serialization).
/// </summary>
[JsonSourceGenerationOptions(PropertyNamingPolicy = JsonKnownNamingPolicy.CamelCase)]
[JsonSerializable(typeof(ProcessedNotificationDocument))]
internal sealed partial class SubscriptionsCosmosJsonContext : JsonSerializerContext;
