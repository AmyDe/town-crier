using TownCrier.Application.Notifications;
using TownCrier.Domain.DeviceRegistrations;
using TownCrier.Domain.Notifications;

namespace TownCrier.Application.Tests.Notifications;

internal sealed class SpyPushNotificationSender : IPushNotificationSender
{
    private readonly List<(Notification Notification, IReadOnlyList<DeviceRegistration> Devices)> sent = [];
    private readonly List<(int ApplicationCount, IReadOnlyList<DeviceRegistration> Devices)> digestsSent = [];

    public IReadOnlyList<(Notification Notification, IReadOnlyList<DeviceRegistration> Devices)> Sent => this.sent;

    public IReadOnlyList<(int ApplicationCount, IReadOnlyList<DeviceRegistration> Devices)> DigestsSent => this.digestsSent;

    public Task SendAsync(Notification notification, IReadOnlyList<DeviceRegistration> devices, CancellationToken ct)
    {
        this.sent.Add((notification, devices));
        return Task.CompletedTask;
    }

    public Task SendDigestAsync(int applicationCount, IReadOnlyList<DeviceRegistration> devices, CancellationToken ct)
    {
        this.digestsSent.Add((applicationCount, devices));
        return Task.CompletedTask;
    }
}
