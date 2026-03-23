using TownCrier.Domain.DecisionAlerts;

namespace TownCrier.Infrastructure.DecisionAlerts;

internal sealed class DecisionAlertDocument
{
    public required string Id { get; init; }

    public required string EntityId { get; init; }

    public required string UserId { get; init; }

    public required string ApplicationUid { get; init; }

    public required string ApplicationName { get; init; }

    public required string ApplicationAddress { get; init; }

    public required string Decision { get; init; }

    public required bool PushSent { get; init; }

    public required DateTimeOffset CreatedAt { get; init; }

    public static DecisionAlertDocument FromDomain(DecisionAlert alert)
    {
        ArgumentNullException.ThrowIfNull(alert);

        return new DecisionAlertDocument
        {
            Id = MakeId(alert.UserId, alert.ApplicationUid),
            EntityId = alert.Id,
            UserId = alert.UserId,
            ApplicationUid = alert.ApplicationUid,
            ApplicationName = alert.ApplicationName,
            ApplicationAddress = alert.ApplicationAddress,
            Decision = alert.Decision,
            PushSent = alert.PushSent,
            CreatedAt = alert.CreatedAt,
        };
    }

    public DecisionAlert ToDomain()
    {
        return DecisionAlert.Reconstitute(
            this.EntityId,
            this.UserId,
            this.ApplicationUid,
            this.ApplicationName,
            this.ApplicationAddress,
            this.Decision,
            this.PushSent,
            this.CreatedAt);
    }

    internal static string MakeId(string userId, string applicationUid) => $"{userId}:{applicationUid}";
}
