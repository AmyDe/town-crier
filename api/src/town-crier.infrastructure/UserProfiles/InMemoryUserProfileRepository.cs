using System.Collections.Concurrent;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Infrastructure.UserProfiles;

public sealed class InMemoryUserProfileRepository : IUserProfileRepository
{
    private readonly ConcurrentDictionary<string, UserProfile> store = new();

    public Task<UserProfile?> GetByUserIdAsync(string userId, CancellationToken ct)
    {
        this.store.TryGetValue(userId, out var profile);
        return Task.FromResult(profile);
    }

    public Task<IReadOnlyList<UserProfile>> GetAllByTierCrossPartitionAsync(SubscriptionTier tier, CancellationToken ct)
    {
        var profiles = this.store.Values
            .Where(p => p.Tier == tier)
            .ToList();
        return Task.FromResult<IReadOnlyList<UserProfile>>(profiles);
    }

    public Task<IReadOnlyList<UserProfile>> GetAllByDigestDayCrossPartitionAsync(DayOfWeek digestDay, CancellationToken ct)
    {
        var profiles = this.store.Values
            .Where(p => p.NotificationPreferences.DigestDay == digestDay)
            .ToList();
        return Task.FromResult<IReadOnlyList<UserProfile>>(profiles);
    }

    public Task<UserProfile?> GetByEmailCrossPartitionAsync(string email, CancellationToken ct)
    {
        var profile = this.store.Values
            .FirstOrDefault(p => string.Equals(p.Email, email, StringComparison.OrdinalIgnoreCase));
        return Task.FromResult(profile);
    }

    public Task<UserProfile?> GetByOriginalTransactionIdCrossPartitionAsync(string originalTransactionId, CancellationToken ct)
    {
        var profile = this.store.Values
            .FirstOrDefault(p => p.OriginalTransactionId == originalTransactionId);
        return Task.FromResult(profile);
    }

    public Task SaveAsync(UserProfile profile, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(profile);
        this.store[profile.UserId] = profile;
        return Task.CompletedTask;
    }

    public Task DeleteAsync(string userId, CancellationToken ct)
    {
        this.store.TryRemove(userId, out _);
        return Task.CompletedTask;
    }

    public Task<IReadOnlyList<UserProfile>> GetDormantCrossPartitionAsync(DateTimeOffset cutoff, CancellationToken ct)
    {
        var profiles = this.store.Values
            .Where(p => p.LastActiveAt < cutoff)
            .ToList();
        return Task.FromResult<IReadOnlyList<UserProfile>>(profiles);
    }

    // pageSize and continuationToken are not emulated — returns all matching profiles in one page.
    public Task<UserProfilePage> ListCrossPartitionAsync(
        string? emailSearch, int pageSize, string? continuationToken, CancellationToken ct)
    {
        var profiles = this.store.Values.AsEnumerable();

        if (emailSearch is not null)
        {
            profiles = profiles.Where(p =>
                p.Email is not null &&
                p.Email.Contains(emailSearch, StringComparison.OrdinalIgnoreCase));
        }

        var result = profiles.OrderBy(p => p.Email, StringComparer.OrdinalIgnoreCase).ToList();
        return Task.FromResult(new UserProfilePage(result, null));
    }
}
