using TownCrier.Application.DeviceRegistrations;
using TownCrier.Domain.DeviceRegistrations;

namespace TownCrier.Application.Tests.DeviceRegistrations;

internal sealed class FakeDeviceRegistrationRepository : IDeviceRegistrationRepository
{
    private readonly Dictionary<string, DeviceRegistration> store = [];
    private readonly List<string> deletedTokens = [];

    public int Count => this.store.Count;

    /// <summary>
    /// Gets every token passed to <see cref="DeleteByTokenAsync"/> in call
    /// order. Tests use this to assert that handler-level pruning paths
    /// dispatched the expected removals — even if the token wasn't seeded.
    /// </summary>
    public IReadOnlyList<string> DeletedTokens => this.deletedTokens;

    /// <summary>
    /// Gets the userId argument supplied to the most recent
    /// <see cref="GetByTokenAsync"/> call. Tests assert this is never null,
    /// confirming the query is partitioned by userId.
    /// </summary>
    public string? LastGetByTokenUserId { get; private set; }

    /// <summary>
    /// Gets the userId argument supplied to the most recent
    /// <see cref="DeleteByTokenAsync"/> call. Tests assert this is never null,
    /// confirming the delete is partitioned by userId.
    /// </summary>
    public string? LastDeleteByTokenUserId { get; private set; }

    public Task<DeviceRegistration?> GetByTokenAsync(string userId, string token, CancellationToken ct)
    {
        this.LastGetByTokenUserId = userId;
        this.store.TryGetValue(token, out var registration);
        return Task.FromResult(registration);
    }

    public Task<IReadOnlyList<DeviceRegistration>> GetByUserIdAsync(string userId, CancellationToken ct)
    {
        var registrations = this.store.Values
            .Where(r => r.UserId == userId)
            .ToList();
        return Task.FromResult<IReadOnlyList<DeviceRegistration>>(registrations);
    }

    public Task SaveAsync(DeviceRegistration registration, CancellationToken ct)
    {
        this.store[registration.Token] = registration;
        return Task.CompletedTask;
    }

    public Task DeleteByTokenAsync(string userId, string token, CancellationToken ct)
    {
        this.LastDeleteByTokenUserId = userId;
        this.deletedTokens.Add(token);
        this.store.Remove(token);
        return Task.CompletedTask;
    }

    public Task DeleteAllByUserIdAsync(string userId, CancellationToken ct)
    {
        var tokens = this.store
            .Where(kvp => kvp.Value.UserId == userId)
            .Select(kvp => kvp.Key)
            .ToList();

        foreach (var token in tokens)
        {
            this.store.Remove(token);
        }

        return Task.CompletedTask;
    }

    public DeviceRegistration? GetByToken(string token)
    {
        this.store.TryGetValue(token, out var registration);
        return registration;
    }
}
