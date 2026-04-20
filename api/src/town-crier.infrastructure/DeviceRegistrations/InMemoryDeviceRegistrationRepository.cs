using System.Collections.Concurrent;
using TownCrier.Application.DeviceRegistrations;
using TownCrier.Domain.DeviceRegistrations;

namespace TownCrier.Infrastructure.DeviceRegistrations;

public sealed class InMemoryDeviceRegistrationRepository : IDeviceRegistrationRepository
{
    private readonly ConcurrentDictionary<string, DeviceRegistration> store = new();

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
        ArgumentNullException.ThrowIfNull(registration);
        this.store[registration.Token] = registration;
        return Task.CompletedTask;
    }

    public Task DeleteByTokenAsync(string token, CancellationToken ct)
    {
        this.store.TryRemove(token, out _);
        return Task.CompletedTask;
    }

    public Task DeleteAllByUserIdAsync(string userId, CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);

        var tokensToRemove = this.store
            .Where(kvp => kvp.Value.UserId == userId)
            .Select(kvp => kvp.Key)
            .ToList();

        foreach (var token in tokensToRemove)
        {
            this.store.TryRemove(token, out _);
        }

        return Task.CompletedTask;
    }
}
