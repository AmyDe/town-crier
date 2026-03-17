using TownCrier.Domain.DeviceRegistrations;

namespace TownCrier.Application.DeviceRegistrations;

public interface IDeviceRegistrationRepository
{
    Task<DeviceRegistration?> GetByTokenAsync(string token, CancellationToken ct);

    Task<IReadOnlyList<DeviceRegistration>> GetByUserIdAsync(string userId, CancellationToken ct);

    Task SaveAsync(DeviceRegistration registration, CancellationToken ct);

    Task DeleteByTokenAsync(string token, CancellationToken ct);
}
