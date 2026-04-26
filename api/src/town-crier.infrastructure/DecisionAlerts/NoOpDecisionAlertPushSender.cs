using TownCrier.Application.DecisionAlerts;
using TownCrier.Domain.DecisionAlerts;
using TownCrier.Domain.DeviceRegistrations;

namespace TownCrier.Infrastructure.DecisionAlerts;

/// <summary>
/// No-op fallback registered until an APNS-backed sender lands. The
/// <see cref="DispatchDecisionAlertCommandHandler"/> still records DecisionAlert
/// documents and marks them as not-pushed; bookmark holders see the outcome on
/// next app launch via the existing alerts query path. Mirrors the
/// <c>NoOpPushNotificationSender</c> pattern used by the watch-zone
/// notification pipeline.
/// </summary>
public sealed class NoOpDecisionAlertPushSender : IDecisionAlertPushSender
{
    public Task SendAsync(DecisionAlert alert, IReadOnlyList<DeviceRegistration> devices, CancellationToken ct)
    {
        return Task.CompletedTask;
    }
}
