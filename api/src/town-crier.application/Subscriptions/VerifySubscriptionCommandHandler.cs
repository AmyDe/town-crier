using TownCrier.Application.Auth;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.Subscriptions;
using TownCrier.Domain.UserProfiles;

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

        var profile = await this.repository.GetByUserIdAsync(command.UserId, ct).ConfigureAwait(false)
            ?? throw new UserProfileNotFoundException(
                $"No user profile found for user '{command.UserId}'.");

        // Verify and decode every supplied JWS. A purchase supplies one signed
        // transaction; a restore supplies the device entitlement set, which may
        // contain lapsed transactions. A tampered JWS anywhere fails the call.
        var now = DateTimeOffset.UtcNow;
        SubscriptionTier highestActiveTier = SubscriptionTier.Free;
        DateTimeOffset? highestActiveExpiry = null;
        string? highestActiveOriginalTransactionId = null;

        foreach (var signedTransaction in command.SignedTransactions)
        {
            var json = await this.jwsVerifier.VerifyAndDecodeAsync(signedTransaction, ct)
                .ConfigureAwait(false);

            var transaction = this.decoder.Decode(json);

            if (!string.Equals(transaction.BundleId, this.settings.BundleId, StringComparison.Ordinal))
            {
                throw new ArgumentException(
                    $"Bundle ID mismatch: expected '{this.settings.BundleId}', got '{transaction.BundleId}'.");
            }

            // A restore may legitimately include lapsed transactions — skip them.
            if (transaction.ExpiresDate <= now)
            {
                continue;
            }

            var tier = ProductMapping.ToTier(transaction.ProductId);
            if (tier > highestActiveTier)
            {
                highestActiveTier = tier;
                highestActiveExpiry = transaction.ExpiresDate;
                highestActiveOriginalTransactionId = transaction.OriginalTransactionId;
            }
        }

        if (highestActiveTier == SubscriptionTier.Free)
        {
            // No active transaction across the supplied set — the user is Free.
            profile.ExpireSubscription();
        }
        else
        {
            profile.LinkOriginalTransactionId(highestActiveOriginalTransactionId!);
            profile.ActivateSubscription(highestActiveTier, highestActiveExpiry!.Value);
        }

        await this.repository.SaveAsync(profile, ct).ConfigureAwait(false);
        await this.auth0Client.UpdateSubscriptionTierAsync(
            profile.UserId, profile.Tier.ToString(), ct).ConfigureAwait(false);

        return VerifySubscriptionResult.FromProfile(profile);
    }
}
