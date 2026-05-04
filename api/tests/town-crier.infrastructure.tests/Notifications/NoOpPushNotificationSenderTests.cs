using TownCrier.Domain.DeviceRegistrations;
using TownCrier.Domain.Notifications;
using TownCrier.Infrastructure.Notifications;

namespace TownCrier.Infrastructure.Tests.Notifications;

public sealed class NoOpPushNotificationSenderTests
{
    [Test]
    public async Task Should_ReturnEmptyResult_When_SendAsyncCalled()
    {
        var sender = new NoOpPushNotificationSender();
        var notification = new NotificationBuilder().Build();
        var devices = new List<DeviceRegistration>();

        var result = await sender.SendAsync(notification, devices, totalUnreadCount: 0, CancellationToken.None);

        await Assert.That(result.InvalidTokens).IsEmpty();
    }

    [Test]
    public async Task Should_ReturnEmptyResult_When_SendDigestAsyncCalled()
    {
        var sender = new NoOpPushNotificationSender();
        var devices = new List<DeviceRegistration>();

        var result = await sender.SendDigestAsync(
            applicationCount: 5, totalUnreadCount: 0, devices, CancellationToken.None);

        await Assert.That(result.InvalidTokens).IsEmpty();
    }
}
