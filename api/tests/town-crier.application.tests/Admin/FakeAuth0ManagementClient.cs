using TownCrier.Application.Auth;

namespace TownCrier.Application.Tests.Admin;

internal sealed class FakeAuth0ManagementClient : IAuth0ManagementClient
{
    public List<(string UserId, string Tier)> Updates { get; } = [];

    public Task UpdateSubscriptionTierAsync(string userId, string tier, CancellationToken ct)
    {
        this.Updates.Add((userId, tier));
        return Task.CompletedTask;
    }
}
