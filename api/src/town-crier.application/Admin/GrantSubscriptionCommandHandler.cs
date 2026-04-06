using TownCrier.Application.Auth;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Admin;

public sealed class GrantSubscriptionCommandHandler
{
    private static readonly DateTimeOffset FarFutureExpiry = new(2099, 12, 31, 0, 0, 0, TimeSpan.Zero);

    private readonly IUserProfileRepository repository;
    private readonly IAuth0ManagementClient auth0Client;

    public GrantSubscriptionCommandHandler(IUserProfileRepository repository, IAuth0ManagementClient auth0Client)
    {
        this.repository = repository;
        this.auth0Client = auth0Client;
    }

    public async Task<GrantSubscriptionResult> HandleAsync(GrantSubscriptionCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var profile = await this.repository.GetByEmailAsync(command.Email, ct).ConfigureAwait(false)
            ?? throw new UserProfileNotFoundException($"No user profile found for email '{command.Email}'.");

        if (command.Tier == SubscriptionTier.Free)
        {
            profile.ExpireSubscription();
        }
        else
        {
            profile.ActivateSubscription(command.Tier, FarFutureExpiry);
        }

        await this.repository.SaveAsync(profile, ct).ConfigureAwait(false);
        await this.auth0Client.UpdateSubscriptionTierAsync(profile.UserId, profile.Tier.ToString(), ct)
            .ConfigureAwait(false);

        return new GrantSubscriptionResult(profile.UserId, profile.Email, profile.Tier);
    }
}
