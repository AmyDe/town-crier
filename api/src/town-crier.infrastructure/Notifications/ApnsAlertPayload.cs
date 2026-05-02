using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Notifications;

internal sealed record ApnsAlertPayload(
    [property: JsonPropertyName("aps")] ApnsAlertAps Aps,
    [property: JsonPropertyName("notificationId")] string NotificationId,
    [property: JsonPropertyName("applicationRef")] string ApplicationRef);
