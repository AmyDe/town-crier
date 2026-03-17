namespace TownCrier.Application.Subscriptions;

public interface IAppStoreNotificationValidator
{
    AppStoreNotification? Validate(string signedPayload);
}
