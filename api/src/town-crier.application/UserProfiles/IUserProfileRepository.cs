using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.UserProfiles;

public interface IUserProfileRepository
{
    Task<UserProfile?> GetByUserIdAsync(string userId, CancellationToken ct);

    Task<UserProfile?> GetByEmailAsync(string email, CancellationToken ct);

    Task<IReadOnlyList<UserProfile>> GetAllByTierAsync(SubscriptionTier tier, CancellationToken ct);

    Task<IReadOnlyList<UserProfile>> GetAllByDigestDayAsync(DayOfWeek digestDay, CancellationToken ct);

    Task<UserProfile?> GetByOriginalTransactionIdAsync(string originalTransactionId, CancellationToken ct);

    Task<UserProfilePage> ListAsync(
        string? emailSearch, int pageSize, string? continuationToken, CancellationToken ct);

    /// <summary>
    /// Returns profiles whose LastActiveAt is strictly before the supplied cutoff
    /// (i.e. dormant relative to the retention policy). Used by the daily
    /// DormantAccountCleanup worker to enforce UK GDPR Art. 5(1)(e) storage
    /// limitation — the privacy policy commits to deleting inactive accounts.
    /// </summary>
    Task<IReadOnlyList<UserProfile>> GetDormantAsync(DateTimeOffset cutoff, CancellationToken ct);

    Task SaveAsync(UserProfile profile, CancellationToken ct);

    Task DeleteAsync(string userId, CancellationToken ct);
}
