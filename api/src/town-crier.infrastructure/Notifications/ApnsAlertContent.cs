using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Notifications;

internal sealed record ApnsAlertContent(
    [property: JsonPropertyName("title")] string Title,
    [property: JsonPropertyName("body")] string Body);
