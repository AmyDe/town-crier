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
    public required string WatchZoneId { get; init; }

    [JsonPropertyName("applicationAddress")]
    public required string ApplicationAddress { get; init; }

    [JsonPropertyName("applicationDescription")]
    public required string ApplicationDescription { get; init; }

    [JsonPropertyName("applicationType")]
    public required string ApplicationType { get; init; }

    [JsonPropertyName("authorityId")]
    public required int AuthorityId { get; init; }

    [JsonPropertyName("pushSent")]
    public required bool PushSent { get; init; }

    [JsonPropertyName("createdAt")]
    public required DateTimeOffset CreatedAt { get; init; }

    [JsonPropertyName("ttl")]
    public int Ttl { get; init; } = NinetyDaysInSeconds;

    public static NotificationDocument FromDomain(Notification notification)
    {
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
            PushSent = notification.PushSent,
            CreatedAt = notification.CreatedAt,
            Ttl = NinetyDaysInSeconds,
        };
    }

    public Notification ToDomain()
    {
        return Notification.Reconstitute(
            this.Id,
            this.UserId,
            this.ApplicationName,
            this.WatchZoneId,
            this.ApplicationAddress,
            this.ApplicationDescription,
            this.ApplicationType,
            this.AuthorityId,
            this.PushSent,
            this.CreatedAt);
    }
}
