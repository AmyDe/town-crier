using System.Text.Json.Serialization;
using TownCrier.Domain.NotificationState;

namespace TownCrier.Infrastructure.NotificationState;

/// <summary>
/// Cosmos-persisted document for a per-user notification watermark. The
/// container partitions on <c>/userId</c> and the document id is the same
/// userId, so each user occupies their own logical partition.
/// </summary>
internal sealed class NotificationStateDocument
{
    [JsonPropertyName("id")]
    public required string Id { get; init; }

    [JsonPropertyName("userId")]
    public required string UserId { get; init; }

    [JsonPropertyName("lastReadAt")]
    public required DateTimeOffset LastReadAt { get; init; }

    [JsonPropertyName("version")]
    public required int Version { get; init; }

    public static NotificationStateDocument FromDomain(NotificationStateAggregate state)
    {
        ArgumentNullException.ThrowIfNull(state);

        return new NotificationStateDocument
        {
            // userId is both id and partition key — one document per user.
            Id = state.UserId,
            UserId = state.UserId,
            LastReadAt = state.LastReadAt,
            Version = state.Version,
        };
    }

    public NotificationStateAggregate ToDomain()
    {
        return NotificationStateAggregate.Reconstitute(this.UserId, this.LastReadAt, this.Version);
    }
}
