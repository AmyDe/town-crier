using System.Text.Json.Serialization;
using TownCrier.Domain.Notifications;

namespace TownCrier.Infrastructure.Notifications;

internal sealed class NotificationDocument
{
    private const int NinetyDaysInSeconds = 90 * 24 * 60 * 60;

    [JsonPropertyName("id")]
    public required string Id { get; init; }

    [JsonPropertyName("userId")]
    public required string UserId { get; init; }

    [JsonPropertyName("applicationName")]
    public required string ApplicationName { get; init; }

    [JsonPropertyName("watchZoneId")]
    public required string? WatchZoneId { get; init; }

    [JsonPropertyName("applicationAddress")]
    public required string ApplicationAddress { get; init; }

    [JsonPropertyName("applicationDescription")]
    public required string ApplicationDescription { get; init; }

    [JsonPropertyName("applicationType")]
    public required string? ApplicationType { get; init; }

    [JsonPropertyName("authorityId")]
    public required int AuthorityId { get; init; }

    [JsonPropertyName("decision")]
    public string? Decision { get; init; }

    // Nullable so legacy Cosmos documents predating tc-so3a.3 hydrate as
    // NotificationEventType.NewApplication (the only event type produced before
    // decision-update notifications shipped). The lazy coalesce in ToDomain
    // is the backfill — no separate migration job required.
    [JsonPropertyName("eventType")]
    public string? EventType { get; init; }

    // Nullable for the same reason as EventType — legacy rows hydrate as
    // NotificationSources.Zone (watch-zone matches were the only source
    // before Saved subscriptions shipped).
    [JsonPropertyName("sources")]
    public string? Sources { get; init; }

    [JsonPropertyName("pushSent")]
    public required bool PushSent { get; init; }

    [JsonPropertyName("emailSent")]
    public bool EmailSent { get; init; }

    [JsonPropertyName("createdAt")]
    public required DateTimeOffset CreatedAt { get; init; }

    [JsonPropertyName("ttl")]
    public int Ttl { get; init; } = NinetyDaysInSeconds;

    public static NotificationDocument FromDomain(Notification notification)
    {
        ArgumentNullException.ThrowIfNull(notification);

        return new NotificationDocument
        {
            Id = notification.Id,
            UserId = notification.UserId,
            ApplicationName = notification.ApplicationName,
            WatchZoneId = notification.WatchZoneId,
            ApplicationAddress = notification.ApplicationAddress,
            ApplicationDescription = notification.ApplicationDescription,
            ApplicationType = notification.ApplicationType,
            AuthorityId = notification.AuthorityId,
            Decision = notification.Decision,
            EventType = notification.EventType.ToString(),
            Sources = notification.Sources.ToString(),
            PushSent = notification.PushSent,
            EmailSent = notification.EmailSent,
            CreatedAt = notification.CreatedAt,
            Ttl = NinetyDaysInSeconds,
        };
    }

    public Notification ToDomain()
    {
        // Coalesce nulls with backfill defaults — legacy rows predating tc-so3a.3
        // lack these fields. NewApplication + Zone reflect the only event type
        // and source produced before decision-update notifications shipped.
        var eventType = this.EventType is null
            ? NotificationEventType.NewApplication
            : Enum.Parse<NotificationEventType>(this.EventType);
        var sources = this.Sources is null
            ? NotificationSources.Zone
            : Enum.Parse<NotificationSources>(this.Sources);

        return Notification.Reconstitute(
            this.Id,
            this.UserId,
            this.ApplicationName,
            this.WatchZoneId,
            this.ApplicationAddress,
            this.ApplicationDescription,
            this.ApplicationType,
            this.AuthorityId,
            this.Decision,
            eventType,
            sources,
            this.PushSent,
            this.EmailSent,
            this.CreatedAt);
    }
}
