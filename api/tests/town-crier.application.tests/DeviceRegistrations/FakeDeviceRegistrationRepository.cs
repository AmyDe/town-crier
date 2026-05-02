using TownCrier.Application.DeviceRegistrations;
using TownCrier.Domain.DeviceRegistrations;

namespace TownCrier.Application.Tests.DeviceRegistrations;

internal sealed class FakeDeviceRegistrationRepository : IDeviceRegistrationRepository
{
    private readonly Dictionary<string, DeviceRegistration> store = [];
    private readonly List<string> deletedTokens = [];

    public int Count => this.store.Count;

    /// <summary>
    /// Records every token passed to <see cref="DeleteByTokenAsync"/> in call
    /// order. Tests use this to assert that handler-level pruning paths
    /// dispatched the expected removals — even if the token wasn't seeded.
    /// </summary>
    public IReadOnlyList<string> DeletedTokens => this.deletedTokens;

    public Task<DeviceRegistration?> GetByTokenAsync(string token, CancellationToken ct)
    {
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

    public Task DeleteByTokenAsync(string token, CancellationToken ct)
    {
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
