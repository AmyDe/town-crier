using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.UserProfiles;

public interface IUserProfileRepository
{
    Task<UserProfile?> GetByUserIdAsync(string userId, CancellationToken ct);

    Task<UserProfile?> GetByOriginalTransactionIdAsync(string originalTransactionId, CancellationToken ct);

    Task SaveAsync(UserProfile profile, CancellationToken ct);

    Task DeleteAsync(string userId, CancellationToken ct);
}
