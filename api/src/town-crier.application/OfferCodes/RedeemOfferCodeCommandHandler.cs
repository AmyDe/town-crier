using TownCrier.Application.Auth;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.OfferCodes;

public sealed class RedeemOfferCodeCommandHandler
{
    private readonly IOfferCodeRepository codeRepository;
    private readonly IUserProfileRepository profileRepository;
    private readonly IAuth0ManagementClient auth0Client;
    private readonly TimeProvider timeProvider;

    public RedeemOfferCodeCommandHandler(
        IOfferCodeRepository codeRepository,
        IUserProfileRepository profileRepository,
        IAuth0ManagementClient auth0Client,
        TimeProvider timeProvider)
    {
        this.codeRepository = codeRepository;
        this.profileRepository = profileRepository;
        this.auth0Client = auth0Client;
        this.timeProvider = timeProvider;
    }

    public async Task<RedeemOfferCodeResult> HandleAsync(
        RedeemOfferCodeCommand command,
        CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var canonical = OfferCodeFormat.Normalize(command.Code);

        var code = await this.codeRepository.GetAsync(canonical, ct).ConfigureAwait(false)
            ?? throw new OfferCodeNotFoundException(canonical);

        if (code.IsRedeemed)
        {
            throw new Domain.OfferCodes.OfferCodeAlreadyRedeemedException(canonical);
        }

        var profile = await this.profileRepository.GetByUserIdAsync(command.UserId, ct).ConfigureAwait(false)
            ?? throw new UserProfileNotFoundException($"No user profile found for userId '{command.UserId}'.");

        if (profile.Tier != SubscriptionTier.Free)
        {
            throw new AlreadySubscribedException();
        }

        var now = this.timeProvider.GetUtcNow();
        code.Redeem(command.UserId, now);
        profile.ActivateSubscription(code.Tier, now.AddDays(code.DurationDays));

        await this.codeRepository.SaveAsync(code, ct).ConfigureAwait(false);
        await this.profileRepository.SaveAsync(profile, ct).ConfigureAwait(false);
        await this.auth0Client.UpdateSubscriptionTierAsync(profile.UserId, profile.Tier.ToString(), ct)
            .ConfigureAwait(false);

        return new RedeemOfferCodeResult(profile.Tier, profile.SubscriptionExpiry!.Value);
    }
}
