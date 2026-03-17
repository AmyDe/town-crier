using TownCrier.Domain.DecisionAlerts;
using TownCrier.Domain.DeviceRegistrations;

namespace TownCrier.Application.DecisionAlerts;

public interface IDecisionAlertPushSender
{
    Task SendAsync(DecisionAlert alert, IReadOnlyList<DeviceRegistration> devices, CancellationToken ct);
}
