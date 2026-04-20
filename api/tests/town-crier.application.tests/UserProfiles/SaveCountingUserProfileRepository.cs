using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.UserProfiles;

/// <summary>
/// Fake repository that counts SaveAsync invocations, used to verify the
/// RecordUserActivityCommandHandler deduplicates writes within the 24-hour window.
/// </summary>
internal sealed class SaveCountingUserProfileRepository : IUserProfileRepository
{
    private readonly Dictionary<string, UserProfile> store = [];

    public int SaveCount { get; private set; }

    public void ResetSaveCount() => this.SaveCount = 0;

    public Task<UserProfile?> GetByUserIdAsync(string userId, CancellationToken ct)
    {
        this.store.TryGetValue(userId, out var profile);
        return Task.FromResult(profile);
    }

    public Task<UserProfile?> GetByEmailAsync(string email, CancellationToken ct)
    {
        var profile = this.store.Values
            .FirstOrDefault(p => string.Equals(p.Email, email, StringComparison.OrdinalIgnoreCase));
        return Task.FromResult(profile);
    }

    public Task<IReadOnlyList<UserProfile>> GetAllByTierAsync(SubscriptionTier tier, CancellationToken ct)
    {
        IReadOnlyList<UserProfile> profiles = this.store.Values.Where(p => p.Tier == tier).ToList();
        return Task.FromResult(profiles);
    }

    public Task<IReadOnlyList<UserProfile>> GetAllByDigestDayAsync(DayOfWeek digestDay, CancellationToken ct)
    {
        IReadOnlyList<UserProfile> profiles =
            this.store.Values.Where(p => p.NotificationPreferences.DigestDay == digestDay).ToList();
        return Task.FromResult(profiles);
    }

    public Task<UserProfile?> GetByOriginalTransactionIdAsync(string originalTransactionId, CancellationToken ct)
    {
        var profile = this.store.Values.FirstOrDefault(p => p.OriginalTransactionId == originalTransactionId);
        return Task.FromResult(profile);
    }

    public Task<IReadOnlyList<UserProfile>> GetDormantAsync(DateTimeOffset cutoff, CancellationToken ct)
    {
        IReadOnlyList<UserProfile> profiles = this.store.Values.Where(p => p.LastActiveAt < cutoff).ToList();
        return Task.FromResult(profiles);
    }

    public Task<UserProfilePage> ListAsync(
        string? emailSearch, int pageSize, string? continuationToken, CancellationToken ct)
    {
        IReadOnlyList<UserProfile> profiles = this.store.Values.ToList();
        return Task.FromResult(new UserProfilePage(profiles, null));
    }

    public Task SaveAsync(UserProfile profile, CancellationToken ct)
    {
        this.SaveCount++;
        this.store[profile.UserId] = profile;
        return Task.CompletedTask;
    }

    public Task DeleteAsync(string userId, CancellationToken ct)
    {
        this.store.Remove(userId);
        return Task.CompletedTask;
    }
}
