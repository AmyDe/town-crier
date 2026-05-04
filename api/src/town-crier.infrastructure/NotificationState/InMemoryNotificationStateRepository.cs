using System.Collections.Concurrent;
using TownCrier.Application.NotificationState;
using TownCrier.Domain.NotificationState;

namespace TownCrier.Infrastructure.NotificationState;

/// <summary>
/// In-memory adapter for <see cref="INotificationStateRepository"/>. Used by the
/// test web factory and any non-Cosmos integration scenarios. Stores one
/// aggregate per <c>UserId</c> with upsert semantics that mirror the Cosmos
/// repository — a fresh save replaces any previous version wholesale.
/// </summary>
public sealed class InMemoryNotificationStateRepository : INotificationStateRepository
{
    private readonly ConcurrentDictionary<string, (DateTimeOffset LastReadAt, int Version)> store = new();

    public Task<NotificationStateAggregate?> GetByUserIdAsync(string userId, CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);

        if (!this.store.TryGetValue(userId, out var snapshot))
        {
            return Task.FromResult<NotificationStateAggregate?>(null);
        }

        var aggregate = NotificationStateAggregate.Reconstitute(
            userId, snapshot.LastReadAt, snapshot.Version);
        return Task.FromResult<NotificationStateAggregate?>(aggregate);
    }

    public Task SaveAsync(NotificationStateAggregate state, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(state);
        this.store[state.UserId] = (state.LastReadAt, state.Version);
        return Task.CompletedTask;
    }
}
