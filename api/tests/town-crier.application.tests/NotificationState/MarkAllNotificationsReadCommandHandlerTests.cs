using TownCrier.Application.NotificationState;
using TownCrier.Application.Tests.Notifications;
using TownCrier.Domain.NotificationState;

namespace TownCrier.Application.Tests.NotificationState;

public sealed class MarkAllNotificationsReadCommandHandlerTests
{
    [Test]
    public async Task Should_AdvanceWatermarkToNow_When_StateExists()
    {
        // Arrange — user already has a state with an older watermark.
        var earlier = new DateTimeOffset(2026, 4, 1, 10, 0, 0, TimeSpan.Zero);
        var now = new DateTimeOffset(2026, 4, 5, 12, 0, 0, TimeSpan.Zero);

        var stateRepo = new FakeNotificationStateRepository();
        stateRepo.Seed(NotificationStateAggregate.Reconstitute(
            "auth0|user-123", earlier, version: 4));

        var timeProvider = new FakeTimeProvider(now);
        var handler = new MarkAllNotificationsReadCommandHandler(stateRepo, timeProvider);

        // Act
        await handler.HandleAsync(
            new MarkAllNotificationsReadCommand("auth0|user-123"), CancellationToken.None);

        // Assert — watermark moved to now, version bumped, persisted.
        var saved = await stateRepo.GetByUserIdAsync("auth0|user-123", CancellationToken.None);
        await Assert.That(saved).IsNotNull();
        await Assert.That(saved!.LastReadAt).IsEqualTo(now);
        await Assert.That(saved.Version).IsEqualTo(5);
    }

    [Test]
    public async Task Should_SeedAndMarkAtNow_When_NoStateExists()
    {
        // Arrange — first-touch user invokes Mark-All-Read directly. The handler
        // must create the aggregate seeded at now (version 1) so the call still
        // produces the correct end-state without throwing.
        var now = new DateTimeOffset(2026, 4, 5, 12, 0, 0, TimeSpan.Zero);
        var stateRepo = new FakeNotificationStateRepository();
        var timeProvider = new FakeTimeProvider(now);
        var handler = new MarkAllNotificationsReadCommandHandler(stateRepo, timeProvider);

        // Act
        await handler.HandleAsync(
            new MarkAllNotificationsReadCommand("auth0|user-123"), CancellationToken.None);

        // Assert — fresh aggregate persisted at now, version 1.
        var saved = await stateRepo.GetByUserIdAsync("auth0|user-123", CancellationToken.None);
        await Assert.That(saved).IsNotNull();
        await Assert.That(saved!.LastReadAt).IsEqualTo(now);
        await Assert.That(saved.Version).IsEqualTo(1);
    }
}
