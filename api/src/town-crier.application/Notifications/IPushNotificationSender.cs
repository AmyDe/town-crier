using TownCrier.Domain.DeviceRegistrations;
using TownCrier.Domain.Notifications;

namespace TownCrier.Application.Notifications;

public interface IPushNotificationSender
{
    Task SendAsync(Notification notification, IReadOnlyList<DeviceRegistration> devices, CancellationToken ct);
}
