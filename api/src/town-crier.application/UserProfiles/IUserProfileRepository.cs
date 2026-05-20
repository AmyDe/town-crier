using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.UserProfiles;

public interface IUserProfileRepository
{
    Task<UserProfile?> GetByUserIdAsync(string userId, CancellationToken ct);

    // Cross-partition — used by GrantSubscriptionCommandHandler (admin-only endpoint,
    // excluded from the user-path cross-partition guard per GH#395 "Out of scope").
    Task<UserProfile?> GetByEmailCrossPartitionAsync(string email, CancellationToken ct);

    // Cross-partition — used by GenerateWeeklyDigestsCommandHandler (worker-only).
    Task<IReadOnlyList<UserProfile>> GetAllByTierCrossPartitionAsync(SubscriptionTier tier, CancellationToken ct);

    // Cross-partition — used by GenerateWeeklyDigestsCommandHandler (worker-only).
    Task<IReadOnlyList<UserProfile>> GetAllByDigestDayCrossPartitionAsync(DayOfWeek digestDay, CancellationToken ct);

    // Cross-partition — used by HandleAppStoreNotificationCommandHandler (not registered in web DI).
    Task<UserProfile?> GetByOriginalTransactionIdCrossPartitionAsync(string originalTransactionId, CancellationToken ct);

    // Cross-partition — used by ListUsersQueryHandler (admin-only, excluded from guard).
    Task<UserProfilePage> ListCrossPartitionAsync(
        string? emailSearch, int pageSize, string? continuationToken, CancellationToken ct);

    // Cross-partition — used by DormantAccountCleanupCommandHandler (worker-only).
    // Returns profiles whose LastActiveAt is strictly before the supplied cutoff
    // (i.e. dormant relative to the retention policy). Enforces UK GDPR Art. 5(1)(e)
    // storage limitation — the privacy policy commits to deleting inactive accounts.
    Task<IReadOnlyList<UserProfile>> GetDormantCrossPartitionAsync(DateTimeOffset cutoff, CancellationToken ct);

    Task SaveAsync(UserProfile profile, CancellationToken ct);

    Task DeleteAsync(string userId, CancellationToken ct);
}
