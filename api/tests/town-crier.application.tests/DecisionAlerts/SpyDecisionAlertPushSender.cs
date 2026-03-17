using TownCrier.Application.DecisionAlerts;
using TownCrier.Domain.DecisionAlerts;
using TownCrier.Domain.DeviceRegistrations;

namespace TownCrier.Application.Tests.DecisionAlerts;

internal sealed class SpyDecisionAlertPushSender : IDecisionAlertPushSender
{
    private readonly List<(DecisionAlert Alert, IReadOnlyList<DeviceRegistration> Devices)> sent = [];

    public IReadOnlyList<(DecisionAlert Alert, IReadOnlyList<DeviceRegistration> Devices)> DecisionAlertsSent => this.sent;

    public Task SendAsync(DecisionAlert alert, IReadOnlyList<DeviceRegistration> devices, CancellationToken ct)
    {
        this.sent.Add((alert, devices));
        return Task.CompletedTask;
    }
}
