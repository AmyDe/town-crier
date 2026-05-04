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
}
