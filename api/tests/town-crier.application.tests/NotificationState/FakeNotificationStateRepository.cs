using TownCrier.Application.NotificationState;
using TownCrier.Domain.NotificationState;

namespace TownCrier.Application.Tests.NotificationState;

/// <summary>
/// In-memory fake of <see cref="INotificationStateRepository"/> for application
/// handler tests. Stores one aggregate per userId; <see cref="GetByUserIdAsync"/>
/// returns null for users not yet seeded (mirroring the first-touch path on the
/// real Cosmos adapter).
/// </summary>
internal sealed class FakeNotificationStateRepository : INotificationStateRepository
{
    private readonly Dictionary<string, NotificationStateAggregate> store = new(StringComparer.Ordinal);

    public IReadOnlyDictionary<string, NotificationStateAggregate> All => this.store;

    /// <summary>
    /// Seeds an aggregate without invoking <see cref="SaveAsync"/>. Useful when
    /// a test wants to start with an existing watermark in place.
    /// </summary>
    /// <param name="state">The aggregate to seed.</param>
    public void Seed(NotificationStateAggregate state)
    {
        ArgumentNullException.ThrowIfNull(state);
        this.store[state.UserId] = state;
    }

    public Task<NotificationStateAggregate?> GetByUserIdAsync(string userId, CancellationToken ct)
    {
        return Task.FromResult(this.store.GetValueOrDefault(userId));
    }

    public Task SaveAsync(NotificationStateAggregate state, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(state);
        this.store[state.UserId] = state;
        return Task.CompletedTask;
    }
}
