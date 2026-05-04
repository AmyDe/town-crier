using TownCrier.Application.NotificationState;
using TownCrier.Application.Tests.Notifications;
using TownCrier.Domain.Notifications;
using TownCrier.Domain.NotificationState;

namespace TownCrier.Application.Tests.NotificationState;

public sealed class GetNotificationStateQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnExistingState_When_UserHasWatermark()
    {
        // Arrange — user has marked-all-read at a known instant; one notification
        // arrived after the watermark and one before. Expect totalUnreadCount=1.
        var watermark = new DateTimeOffset(2026, 4, 1, 10, 0, 0, TimeSpan.Zero);
        var stateRepo = new FakeNotificationStateRepository();
        stateRepo.Seed(NotificationStateAggregate.Reconstitute(
            "auth0|user-123", watermark, version: 4));

        var notificationRepo = new FakeNotificationRepository();
        notificationRepo.Seed(BuildNotification(
            "auth0|user-123", "app-A", watermark.AddMinutes(-30)));
        notificationRepo.Seed(BuildNotification(
            "auth0|user-123", "app-B", watermark.AddMinutes(30)));

        var timeProvider = new FakeTimeProvider(watermark.AddHours(2));
        var handler = new GetNotificationStateQueryHandler(stateRepo, notificationRepo, timeProvider);

        // Act
        var result = await handler.HandleAsync(
            new GetNotificationStateQuery("auth0|user-123"), CancellationToken.None);

        // Assert
        await Assert.That(result.LastReadAt).IsEqualTo(watermark);
        await Assert.That(result.Version).IsEqualTo(4);
        await Assert.That(result.TotalUnreadCount).IsEqualTo(1);
    }

    [Test]
    public async Task Should_SeedAtNow_When_UserHasNoWatermark()
    {
        // Arrange — first-touch path: no document yet. Per spec Pre-Resolved
        // Decision #13 ("clean slate"), the server seeds lastReadAt = now and
        // persists it so the user's existing notifications all count as read.
        var now = new DateTimeOffset(2026, 4, 5, 12, 0, 0, TimeSpan.Zero);
        var stateRepo = new FakeNotificationStateRepository();
        var notificationRepo = new FakeNotificationRepository();
        notificationRepo.Seed(BuildNotification(
            "auth0|user-123", "app-A", now.AddDays(-3)));

        var timeProvider = new FakeTimeProvider(now);
        var handler = new GetNotificationStateQueryHandler(stateRepo, notificationRepo, timeProvider);

        // Act
        var result = await handler.HandleAsync(
            new GetNotificationStateQuery("auth0|user-123"), CancellationToken.None);

        // Assert — watermark seeded at now, version=1, all prior notifications read.
        await Assert.That(result.LastReadAt).IsEqualTo(now);
        await Assert.That(result.Version).IsEqualTo(1);
        await Assert.That(result.TotalUnreadCount).IsEqualTo(0);

        // Assert — the seed is persisted so subsequent reads see the same state.
        var persisted = await stateRepo.GetByUserIdAsync("auth0|user-123", CancellationToken.None);
        await Assert.That(persisted).IsNotNull();
        await Assert.That(persisted!.LastReadAt).IsEqualTo(now);
        await Assert.That(persisted.Version).IsEqualTo(1);
    }

    private static Notification BuildNotification(
        string userId, string applicationUid, DateTimeOffset createdAt)
    {
        return Notification.Create(
            userId: userId,
            applicationUid: applicationUid,
            applicationName: "Test app",
            watchZoneId: "zone-1",
            applicationAddress: "1 High St",
            applicationDescription: "Test description",
            applicationType: "Full",
            authorityId: 42,
            now: createdAt);
    }
}
