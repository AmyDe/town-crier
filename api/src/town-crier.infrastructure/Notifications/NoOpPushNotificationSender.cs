using TownCrier.Application.Notifications;
using TownCrier.Domain.DeviceRegistrations;
using TownCrier.Domain.Notifications;

namespace TownCrier.Infrastructure.Notifications;

public sealed class NoOpPushNotificationSender : IPushNotificationSender
{
    public Task SendAsync(Notification notification, IReadOnlyList<DeviceRegistration> devices, CancellationToken ct)
    {
        return Task.CompletedTask;
    }
}
