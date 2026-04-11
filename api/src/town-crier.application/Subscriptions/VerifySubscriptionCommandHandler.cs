using TownCrier.Application.Auth;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.Subscriptions;

namespace TownCrier.Application.Subscriptions;

public sealed class VerifySubscriptionCommandHandler
{
    private readonly IAppleJwsVerifier jwsVerifier;
    private readonly ITransactionDecoder decoder;
    private readonly IUserProfileRepository repository;
    private readonly IAuth0ManagementClient auth0Client;
    private readonly AppleSettings settings;

    public VerifySubscriptionCommandHandler(
        IAppleJwsVerifier jwsVerifier,
        ITransactionDecoder decoder,
        IUserProfileRepository repository,
        IAuth0ManagementClient auth0Client,
        AppleSettings settings)
    {
        this.jwsVerifier = jwsVerifier;
        this.decoder = decoder;
        this.repository = repository;
        this.auth0Client = auth0Client;
        this.settings = settings;
    }

    public async Task<VerifySubscriptionResult> HandleAsync(
        VerifySubscriptionCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var json = await this.jwsVerifier.VerifyAndDecodeAsync(command.SignedTransaction, ct)
            .ConfigureAwait(false);

        var transaction = this.decoder.Decode(json);

        if (!string.Equals(transaction.BundleId, this.settings.BundleId, StringComparison.Ordinal))
        {
            throw new ArgumentException(
                $"Bundle ID mismatch: expected '{this.settings.BundleId}', got '{transaction.BundleId}'.");
        }

        var tier = ProductMapping.ToTier(transaction.ProductId);

        var profile = await this.repository.GetByUserIdAsync(command.UserId, ct).ConfigureAwait(false)
            ?? throw new UserProfileNotFoundException(
                $"No user profile found for user '{command.UserId}'.");

        profile.LinkOriginalTransactionId(transaction.OriginalTransactionId);
        profile.ActivateSubscription(tier, transaction.ExpiresDate);

        await this.repository.SaveAsync(profile, ct).ConfigureAwait(false);
        await this.auth0Client.UpdateSubscriptionTierAsync(
            profile.UserId, profile.Tier.ToString(), ct).ConfigureAwait(false);

        return VerifySubscriptionResult.FromProfile(profile);
    }
}
