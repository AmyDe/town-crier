using TownCrier.Domain.DeviceRegistrations;

namespace TownCrier.Application.DeviceRegistrations;

public interface IDeviceRegistrationRepository
{
    /// <summary>
    /// Returns the device registration for the given user and token, scoped to the
    /// user's partition. Never executes a cross-partition query.
    /// </summary>
    /// <param name="userId">The user's partition key (JWT <c>sub</c> claim).</param>
    /// <param name="token">The APNs/FCM device token.</param>
    /// <param name="ct">Cancellation token.</param>
    /// <returns>The matching registration, or <c>null</c> if not found.</returns>
    Task<DeviceRegistration?> GetByTokenAsync(string userId, string token, CancellationToken ct);

    Task<IReadOnlyList<DeviceRegistration>> GetByUserIdAsync(string userId, CancellationToken ct);

    Task SaveAsync(DeviceRegistration registration, CancellationToken ct);

    /// <summary>
    /// Deletes the device registration for the given user and token, scoped to the
    /// user's partition. Never executes a cross-partition query.
    /// </summary>
    /// <param name="userId">The user's partition key (JWT <c>sub</c> claim).</param>
    /// <param name="token">The APNs/FCM device token.</param>
    /// <param name="ct">Cancellation token.</param>
    /// <returns>A <see cref="Task"/> representing the asynchronous operation.</returns>
    Task DeleteByTokenAsync(string userId, string token, CancellationToken ct);

    Task DeleteAllByUserIdAsync(string userId, CancellationToken ct);
}
