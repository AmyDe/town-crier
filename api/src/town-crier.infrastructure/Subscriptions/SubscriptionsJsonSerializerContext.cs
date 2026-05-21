using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Subscriptions;

/// <summary>
/// Source-generated JSON metadata for Apple JWS parsing — keeps the
/// infrastructure layer Native AOT-compatible (no reflection-based
/// serialization).
/// </summary>
[JsonSourceGenerationOptions(WriteIndented = false)]
[JsonSerializable(typeof(JwsHeader))]
[JsonSerializable(typeof(AppleTransactionPayload))]
[JsonSerializable(typeof(AppleNotificationPayload))]
[JsonSerializable(typeof(AppleNotificationData))]
internal sealed partial class SubscriptionsJsonSerializerContext : JsonSerializerContext;
