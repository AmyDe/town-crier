using TownCrier.Domain.DecisionAlerts;

namespace TownCrier.Application.DecisionAlerts;

public interface IDecisionAlertRepository
{
    Task<DecisionAlert?> GetByUserAndApplicationAsync(string userId, string applicationUid, CancellationToken ct);

    Task SaveAsync(DecisionAlert alert, CancellationToken ct);

    Task DeleteAllByUserIdAsync(string userId, CancellationToken ct);
}
