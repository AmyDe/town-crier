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

    public Task<IReadOnlyList<UserProfile>> GetAllByTierAsync(SubscriptionTier tier, CancellationToken ct)
    {
        var profiles = this.store.Values
            .Where(p => p.Tier == tier)
            .ToList();
        return Task.FromResult<IReadOnlyList<UserProfile>>(profiles);
    }

    public Task<IReadOnlyList<UserProfile>> GetAllByDigestDayAsync(DayOfWeek digestDay, CancellationToken ct)
    {
        var profiles = this.store.Values
            .Where(p => p.NotificationPreferences.DigestDay == digestDay)
            .ToList();
        return Task.FromResult<IReadOnlyList<UserProfile>>(profiles);
    }

    public Task<UserProfile?> GetByEmailAsync(string email, CancellationToken ct)
    {
        var profile = this.store.Values
            .FirstOrDefault(p => string.Equals(p.Email, email, StringComparison.OrdinalIgnoreCase));
        return Task.FromResult(profile);
    }

    public Task<UserProfile?> GetByOriginalTransactionIdAsync(string originalTransactionId, CancellationToken ct)
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

    public Task<UserProfilePage> ListAsync(
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
