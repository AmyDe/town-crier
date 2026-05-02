using TownCrier.Domain.DeviceRegistrations;
using TownCrier.Domain.Notifications;

namespace TownCrier.Application.Notifications;

public interface IPushNotificationSender
{
    Task<PushSendResult> SendAsync(Notification notification, IReadOnlyList<DeviceRegistration> devices, CancellationToken ct);

    Task<PushSendResult> SendDigestAsync(int applicationCount, IReadOnlyList<DeviceRegistration> devices, CancellationToken ct);
}
