namespace TownCrier.Application.Auth;

public interface IAuth0ManagementClient
{
    Task UpdateSubscriptionTierAsync(string userId, string tier, CancellationToken ct);
}
