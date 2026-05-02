using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Notifications;

internal sealed record ApnsDigestAps(
    [property: JsonPropertyName("alert")] ApnsAlertContent Alert,
    [property: JsonPropertyName("sound")] string Sound,
    [property: JsonPropertyName("badge")] int Badge);
