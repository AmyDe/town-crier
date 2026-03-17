using TownCrier.Application.DeviceRegistrations;
using TownCrier.Domain.DeviceRegistrations;

namespace TownCrier.Application.Tests.DeviceRegistrations;

internal sealed class FakeDeviceRegistrationRepository : IDeviceRegistrationRepository
{
    private readonly Dictionary<string, DeviceRegistration> store = [];

    public int Count => this.store.Count;

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
        this.store.Remove(token);
        return Task.CompletedTask;
    }

    public DeviceRegistration? GetByToken(string token)
    {
        this.store.TryGetValue(token, out var registration);
        return registration;
    }
}
