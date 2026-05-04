using TownCrier.Domain.NotificationState;

namespace TownCrier.Domain.Tests.NotificationState;

public sealed class NotificationStateTests
{
    private static readonly DateTimeOffset Now = new(2026, 5, 4, 10, 0, 0, TimeSpan.Zero);

    [Test]
    public async Task Should_InitializeWithLastReadAt_When_Created()
    {
        // Arrange — fresh state for a user that has never marked anything read.
        // Endpoint adapter (tc-1nsa.2) seeds via Create on first GET.
        var state = NotificationStateAggregate.Create("auth0|user-1", Now);

        // Assert
        await Assert.That(state.UserId).IsEqualTo("auth0|user-1");
        await Assert.That(state.LastReadAt).IsEqualTo(Now);
        await Assert.That(state.Version).IsEqualTo(1);
    }

    [Test]
    public async Task Should_RejectEmptyUserId_When_Created()
    {
        // Defensive: the aggregate is keyed on userId both as id and partition key
        // on the Cosmos container — an empty string would silently merge state
        // across "anonymous" callers.
        Assert.Throws<ArgumentException>(() => NotificationStateAggregate.Create(string.Empty, Now));
        await Task.CompletedTask;
    }

    [Test]
    public async Task Should_AdvanceWatermarkAndBumpVersion_When_MarkAllReadAt()
    {
        // Arrange — typical mark-all-read flow: state was seeded yesterday,
        // user taps the trailing toolbar action, watermark jumps to "now".
        var yesterday = Now.AddDays(-1);
        var state = NotificationStateAggregate.Create("auth0|user-1", yesterday);

        // Act
        state.MarkAllReadAt(Now);

        // Assert
        await Assert.That(state.LastReadAt).IsEqualTo(Now);
        await Assert.That(state.Version).IsEqualTo(2);
    }

    [Test]
    public async Task Should_AcceptStaleClock_When_MarkAllReadAt()
    {
        // Mark-all-read is the user-driven "I'm caught up" action — the spec
        // takes the supplied "now" as authoritative even if it lands behind
        // the previous watermark (e.g. a device with a skewed clock). This is
        // distinct from AdvanceTo, which is monotonic.
        var state = NotificationStateAggregate.Create("auth0|user-1", Now);
        var earlier = Now.AddHours(-1);

        // Act
        state.MarkAllReadAt(earlier);

        // Assert — latest call wins, version still bumps so consumers detect the change.
        await Assert.That(state.LastReadAt).IsEqualTo(earlier);
        await Assert.That(state.Version).IsEqualTo(2);
    }

    [Test]
    public async Task Should_AdvanceWatermark_When_AdvanceToFutureInstant()
    {
        // Arrange — push-tap flow: the user opens a notification with createdAt
        // strictly after the current watermark, so the watermark advances to
        // that exact instant.
        var state = NotificationStateAggregate.Create("auth0|user-1", Now);
        var pushCreatedAt = Now.AddMinutes(5);

        // Act
        var advanced = state.AdvanceTo(pushCreatedAt);

        // Assert
        await Assert.That(advanced).IsTrue();
        await Assert.That(state.LastReadAt).IsEqualTo(pushCreatedAt);
        await Assert.That(state.Version).IsEqualTo(2);
    }

    [Test]
    public async Task Should_LeaveWatermarkUntouched_When_AdvanceToOlderInstant()
    {
        // Monotonic guarantee per spec Pre-Resolved Decision #11: the server
        // never moves the watermark backwards on advance. A push tapped from
        // an old in-tray entry must be a no-op.
        var state = NotificationStateAggregate.Create("auth0|user-1", Now);
        var older = Now.AddHours(-2);

        // Act
        var advanced = state.AdvanceTo(older);

        // Assert — version stays put so consumers can rely on it as a change marker.
        await Assert.That(advanced).IsFalse();
        await Assert.That(state.LastReadAt).IsEqualTo(Now);
        await Assert.That(state.Version).IsEqualTo(1);
    }

    [Test]
    public async Task Should_LeaveWatermarkUntouched_When_AdvanceToSameInstant()
    {
        // Equal instants are a no-op too — strictly forward (>) per the
        // "monotonic" wording, so duplicated push-taps don't churn the version.
        var state = NotificationStateAggregate.Create("auth0|user-1", Now);

        // Act
        var advanced = state.AdvanceTo(Now);

        // Assert
        await Assert.That(advanced).IsFalse();
        await Assert.That(state.LastReadAt).IsEqualTo(Now);
        await Assert.That(state.Version).IsEqualTo(1);
    }

    [Test]
    public async Task Should_RoundTripFields_When_Reconstituted()
    {
        // Reconstitute is the persistence boundary: the Cosmos repository hydrates
        // a stored document back into the aggregate without bumping version or
        // resetting watermark. Used by GetByUserIdAsync.
        var state = NotificationStateAggregate.Reconstitute("auth0|user-1", Now, version: 7);

        // Assert
        await Assert.That(state.UserId).IsEqualTo("auth0|user-1");
        await Assert.That(state.LastReadAt).IsEqualTo(Now);
        await Assert.That(state.Version).IsEqualTo(7);
    }
}
