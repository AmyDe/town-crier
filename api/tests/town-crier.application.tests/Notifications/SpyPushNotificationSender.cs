using TownCrier.Application.Notifications;
using TownCrier.Domain.DeviceRegistrations;
using TownCrier.Domain.Notifications;

namespace TownCrier.Application.Tests.Notifications;

internal sealed class SpyPushNotificationSender : IPushNotificationSender
{
    private readonly List<(Notification Notification, IReadOnlyList<DeviceRegistration> Devices)> sent = [];

    public IReadOnlyList<(Notification Notification, IReadOnlyList<DeviceRegistration> Devices)> Sent => this.sent;

    public Task SendAsync(Notification notification, IReadOnlyList<DeviceRegistration> devices, CancellationToken ct)
    {
        this.sent.Add((notification, devices));
        return Task.CompletedTask;
    }
}
