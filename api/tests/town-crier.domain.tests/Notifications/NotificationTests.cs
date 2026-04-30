using TownCrier.Domain.Notifications;

namespace TownCrier.Domain.Tests.Notifications;

public sealed class NotificationTests
{
    private static readonly DateTimeOffset Now = new(2026, 4, 29, 10, 0, 0, TimeSpan.Zero);

    [Test]
    public async Task Should_StoreDecision_When_Created()
    {
        var notification = Notification.Create(
            userId: "user-1",
            applicationName: "APP/2026/0001",
            watchZoneId: "zone-1",
            applicationAddress: "123 High Street",
            applicationDescription: "Single storey rear extension",
            applicationType: "Householder",
            authorityId: 42,
            now: Now,
            decision: "Permitted");

        await Assert.That(notification.Decision).IsEqualTo("Permitted");
    }

    [Test]
    public async Task Should_DefaultDecisionToNull_When_NotProvided()
    {
        var notification = Notification.Create(
            userId: "user-1",
            applicationName: "APP/2026/0001",
            watchZoneId: "zone-1",
            applicationAddress: "123 High Street",
            applicationDescription: "Single storey rear extension",
            applicationType: "Householder",
            authorityId: 42,
            now: Now);

        await Assert.That(notification.Decision).IsNull();
    }

    [Test]
    public async Task Should_AllowNullWatchZoneId_When_Created()
    {
        var notification = Notification.Create(
            userId: "user-1",
            applicationName: "APP/2026/0001",
            watchZoneId: null,
            applicationAddress: "123 High Street",
            applicationDescription: "Single storey rear extension",
            applicationType: "Householder",
            authorityId: 42,
            now: Now);

        await Assert.That(notification.WatchZoneId).IsNull();
    }
}
