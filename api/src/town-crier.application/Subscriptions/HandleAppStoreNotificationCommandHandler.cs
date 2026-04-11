using TownCrier.Application.Auth;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.Subscriptions;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Subscriptions;

public sealed class HandleAppStoreNotificationCommandHandler
{
    private readonly IAppleJwsVerifier jwsVerifier;
    private readonly INotificationDecoder notificationDecoder;
    private readonly ITransactionDecoder transactionDecoder;
    private readonly IUserProfileRepository repository;
    private readonly IAuth0ManagementClient auth0Client;
    private readonly INotificationIdempotencyStore idempotencyStore;

    public HandleAppStoreNotificationCommandHandler(
        IAppleJwsVerifier jwsVerifier,
        INotificationDecoder notificationDecoder,
        ITransactionDecoder transactionDecoder,
        IUserProfileRepository repository,
        IAuth0ManagementClient auth0Client,
        INotificationIdempotencyStore idempotencyStore)
    {
        this.jwsVerifier = jwsVerifier;
        this.notificationDecoder = notificationDecoder;
        this.transactionDecoder = transactionDecoder;
        this.repository = repository;
        this.auth0Client = auth0Client;
        this.idempotencyStore = idempotencyStore;
    }

    public async Task HandleAsync(HandleAppStoreNotificationCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        // Verify and decode the outer JWS
        var outerJson = await this.jwsVerifier.VerifyAndDecodeAsync(command.SignedPayload, ct)
            .ConfigureAwait(false);
        var notification = this.notificationDecoder.Decode(outerJson);

        // Idempotency check
        if (await this.idempotencyStore.IsProcessedAsync(notification.NotificationUuid, ct)
            .ConfigureAwait(false))
        {
            return;
        }

        // Verify and decode the inner JWS (transaction info)
        var txnJson = await this.jwsVerifier.VerifyAndDecodeAsync(
            notification.SignedTransactionInfo, ct).ConfigureAwait(false);
        var transaction = this.transactionDecoder.Decode(txnJson);

        // Look up user by original transaction ID
        var profile = await this.repository.GetByOriginalTransactionIdAsync(
            transaction.OriginalTransactionId, ct).ConfigureAwait(false);

        if (profile is not null)
        {
            var stateChanged = ApplyNotification(
                profile, notification, transaction);

            if (stateChanged)
            {
                await this.repository.SaveAsync(profile, ct).ConfigureAwait(false);
                await this.auth0Client.UpdateSubscriptionTierAsync(
                    profile.UserId, profile.Tier.ToString(), ct).ConfigureAwait(false);
            }
        }

        await this.idempotencyStore.MarkProcessedAsync(notification.NotificationUuid, ct)
            .ConfigureAwait(false);
    }

    private static bool ApplyNotification(
        UserProfile profile,
        DecodedNotification notification,
        DecodedTransaction transaction)
    {
        switch (notification.NotificationType)
        {
            case "SUBSCRIBED":
            case "OFFER_REDEEMED":
                {
                    var tier = ProductMapping.ToTier(transaction.ProductId);
                    profile.ActivateSubscription(tier, transaction.ExpiresDate);
                    return true;
                }

            case "DID_RENEW":
                {
                    profile.RenewSubscription(transaction.ExpiresDate);
                    return true;
                }

            case "DID_CHANGE_RENEWAL_PREF":
                {
                    if (string.Equals(notification.Subtype, "UPGRADE", StringComparison.Ordinal))
                    {
                        var tier = ProductMapping.ToTier(transaction.ProductId);
                        profile.ActivateSubscription(tier, transaction.ExpiresDate);
                        return true;
                    }

                    // DOWNGRADE: no state change — takes effect at next renewal
                    return false;
                }

            case "DID_FAIL_TO_RENEW":
                {
                    if (string.Equals(notification.Subtype, "GRACE_PERIOD", StringComparison.Ordinal))
                    {
                        // Use the expires date from the transaction as the grace period end
                        profile.EnterGracePeriod(transaction.ExpiresDate);
                        return true;
                    }

                    profile.ExpireSubscription();
                    return true;
                }

            case "EXPIRED":
            case "GRACE_PERIOD_EXPIRED":
            case "REFUND":
            case "REVOKE":
                {
                    profile.ExpireSubscription();
                    return true;
                }

            default:
                // TEST, PRICE_INCREASE, REFUND_DECLINED, etc. — log and ignore
                return false;
        }
    }
}
