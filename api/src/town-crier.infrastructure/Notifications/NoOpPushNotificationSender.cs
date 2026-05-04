using TownCrier.Application.Notifications;
using TownCrier.Domain.DeviceRegistrations;
using TownCrier.Domain.Notifications;

namespace TownCrier.Infrastructure.Notifications;

public sealed class NoOpPushNotificationSender : IPushNotificationSender
{
    public Task<PushSendResult> SendAsync(
        Notification notification,
        IReadOnlyList<DeviceRegistration> devices,
        int totalUnreadCount,
        CancellationToken ct)
    {
        return Task.FromResult(PushSendResult.Empty);
    }

    public Task<PushSendResult> SendDigestAsync(
        int applicationCount,
        int totalUnreadCount,
        IReadOnlyList<DeviceRegistration> devices,
        CancellationToken ct)
    {
        return Task.FromResult(PushSendResult.Empty);
    }
}
