using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Subscriptions;

/// <summary>
/// The decoded JSON payload of an Apple App Store Server Notification v2
/// (the <c>responseBodyV2DecodedPayload</c> shape). The signed JWS strings
/// for the transaction and renewal info are nested under <c>data</c>.
/// </summary>
internal sealed record AppleNotificationPayload(
    [property: JsonPropertyName("notificationType")] string? NotificationType,
    [property: JsonPropertyName("subtype")] string? Subtype,
    [property: JsonPropertyName("notificationUUID")] string? NotificationUuid,
    [property: JsonPropertyName("data")] AppleNotificationData? Data);
