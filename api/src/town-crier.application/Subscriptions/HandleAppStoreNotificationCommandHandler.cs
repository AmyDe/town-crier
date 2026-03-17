using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Subscriptions;

public sealed class HandleAppStoreNotificationCommandHandler
{
    private readonly IAppStoreNotificationValidator validator;
    private readonly IUserProfileRepository repository;

    public HandleAppStoreNotificationCommandHandler(
        IAppStoreNotificationValidator validator,
        IUserProfileRepository repository)
    {
        this.validator = validator;
        this.repository = repository;
    }

    public async Task<HandleAppStoreNotificationResult> HandleAsync(
        HandleAppStoreNotificationCommand command,
        CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var notification = this.validator.Validate(command.SignedPayload);
        if (notification is null)
        {
            return new HandleAppStoreNotificationResult(NotificationOutcome.InvalidSignature);
        }

        var profile = await this.repository
            .GetByOriginalTransactionIdAsync(notification.OriginalTransactionId, ct)
            .ConfigureAwait(false);

        if (profile is null)
        {
            return new HandleAppStoreNotificationResult(NotificationOutcome.UserNotFound);
        }

        ApplyNotification(profile, notification);

        await this.repository.SaveAsync(profile, ct).ConfigureAwait(false);

        return new HandleAppStoreNotificationResult(NotificationOutcome.Processed);
    }

    private static void ApplyNotification(
        UserProfile profile,
        AppStoreNotification notification)
    {
        switch (notification.NotificationType)
        {
            case AppStoreNotificationType.Subscribed:
            case AppStoreNotificationType.DidRenew:
                if (notification.ExpiresDate.HasValue)
                {
                    if (notification.NotificationType == AppStoreNotificationType.Subscribed)
                    {
                        profile.ActivateSubscription(notification.Tier, notification.ExpiresDate.Value);
                    }
                    else
                    {
                        profile.RenewSubscription(notification.ExpiresDate.Value);
                    }
                }

                break;

            case AppStoreNotificationType.Expired:
            case AppStoreNotificationType.Refund:
            case AppStoreNotificationType.GracePeriodExpired:
                profile.ExpireSubscription();
                break;
        }
    }
}
