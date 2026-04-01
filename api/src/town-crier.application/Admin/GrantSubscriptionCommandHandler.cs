using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Admin;

public sealed class GrantSubscriptionCommandHandler
{
    private static readonly DateTimeOffset FarFutureExpiry = new(2099, 12, 31, 0, 0, 0, TimeSpan.Zero);

    private readonly IUserProfileRepository repository;

    public GrantSubscriptionCommandHandler(IUserProfileRepository repository)
    {
        this.repository = repository;
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

        return new GrantSubscriptionResult(profile.UserId, profile.Email, profile.Tier);
    }
}
