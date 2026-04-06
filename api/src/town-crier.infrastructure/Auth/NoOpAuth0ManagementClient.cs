using TownCrier.Application.Auth;

namespace TownCrier.Infrastructure.Auth;

public sealed class NoOpAuth0ManagementClient : IAuth0ManagementClient
{
    public Task UpdateSubscriptionTierAsync(string userId, string tier, CancellationToken ct)
    {
        return Task.CompletedTask;
    }
}
