using TownCrier.Application.NotificationState;
using TownCrier.Application.Tests.Notifications;
using TownCrier.Domain.NotificationState;

namespace TownCrier.Application.Tests.NotificationState;

public sealed class AdvanceNotificationStateCommandHandlerTests
{
    [Test]
    public async Task Should_AdvanceWatermark_When_AsOfIsLater()
    {
        // Arrange — push-tap on a notification newer than the current watermark.
        var watermark = new DateTimeOffset(2026, 4, 1, 10, 0, 0, TimeSpan.Zero);
        var asOf = watermark.AddHours(2);

        var stateRepo = new FakeNotificationStateRepository();
        stateRepo.Seed(NotificationStateAggregate.Reconstitute(
            "auth0|user-123", watermark, version: 4));

        var timeProvider = new FakeTimeProvider(asOf.AddDays(1));
        var handler = new AdvanceNotificationStateCommandHandler(stateRepo, timeProvider);

        // Act
        await handler.HandleAsync(
            new AdvanceNotificationStateCommand("auth0|user-123", asOf), CancellationToken.None);

        // Assert — watermark moved, version bumped, persisted.
        var saved = await stateRepo.GetByUserIdAsync("auth0|user-123", CancellationToken.None);
        await Assert.That(saved!.LastReadAt).IsEqualTo(asOf);
        await Assert.That(saved.Version).IsEqualTo(5);
    }

    [Test]
    public async Task Should_NoOp_When_AsOfIsEarlierThanWatermark()
    {
        // Arrange — older notification tapped after Mark-All-Read; spec says
        // server must never move the watermark backwards (Pre-Resolved Decision
        // #11). Version must not bump.
        var watermark = new DateTimeOffset(2026, 4, 5, 10, 0, 0, TimeSpan.Zero);
        var asOf = watermark.AddHours(-3);

        var stateRepo = new FakeNotificationStateRepository();
        stateRepo.Seed(NotificationStateAggregate.Reconstitute(
            "auth0|user-123", watermark, version: 7));

        var timeProvider = new FakeTimeProvider(watermark.AddHours(1));
        var handler = new AdvanceNotificationStateCommandHandler(stateRepo, timeProvider);

        // Act
        await handler.HandleAsync(
            new AdvanceNotificationStateCommand("auth0|user-123", asOf), CancellationToken.None);

        // Assert — unchanged.
        var saved = await stateRepo.GetByUserIdAsync("auth0|user-123", CancellationToken.None);
        await Assert.That(saved!.LastReadAt).IsEqualTo(watermark);
        await Assert.That(saved.Version).IsEqualTo(7);
    }

    [Test]
    public async Task Should_NoOp_When_AsOfEqualsWatermark()
    {
        // Arrange — equality is also a no-op per AdvanceTo's strict-greater-than
        // contract. Boundary instant counts as already-read.
        var watermark = new DateTimeOffset(2026, 4, 5, 10, 0, 0, TimeSpan.Zero);

        var stateRepo = new FakeNotificationStateRepository();
        stateRepo.Seed(NotificationStateAggregate.Reconstitute(
            "auth0|user-123", watermark, version: 9));

        var timeProvider = new FakeTimeProvider(watermark.AddHours(1));
        var handler = new AdvanceNotificationStateCommandHandler(stateRepo, timeProvider);

        // Act
        await handler.HandleAsync(
            new AdvanceNotificationStateCommand("auth0|user-123", watermark), CancellationToken.None);

        // Assert — unchanged.
        var saved = await stateRepo.GetByUserIdAsync("auth0|user-123", CancellationToken.None);
        await Assert.That(saved!.LastReadAt).IsEqualTo(watermark);
        await Assert.That(saved.Version).IsEqualTo(9);
    }

    [Test]
    public async Task Should_SeedAndAdvance_When_NoStateExists()
    {
        // Arrange — first-touch user taps a push. Seed at now, then advance to
        // asOf if it lies after that seed (it usually does — push notifications
        // arrive in the same window the device picks them up).
        var now = new DateTimeOffset(2026, 4, 5, 10, 0, 0, TimeSpan.Zero);
        var asOf = now.AddSeconds(30);

        var stateRepo = new FakeNotificationStateRepository();
        var timeProvider = new FakeTimeProvider(now);
        var handler = new AdvanceNotificationStateCommandHandler(stateRepo, timeProvider);

        // Act
        await handler.HandleAsync(
            new AdvanceNotificationStateCommand("auth0|user-123", asOf), CancellationToken.None);

        // Assert — fresh aggregate persisted at asOf, version 2 (one for Create,
        // one for AdvanceTo).
        var saved = await stateRepo.GetByUserIdAsync("auth0|user-123", CancellationToken.None);
        await Assert.That(saved!.LastReadAt).IsEqualTo(asOf);
        await Assert.That(saved.Version).IsEqualTo(2);
    }
}
