using TownCrier.Application.Subscriptions;

namespace TownCrier.Application.Tests.Subscriptions;

internal sealed class FakeAppStoreNotificationValidator : IAppStoreNotificationValidator
{
    private readonly Dictionary<string, AppStoreNotification> validPayloads = [];

    public void AddValidPayload(string signedPayload, AppStoreNotification notification)
    {
        this.validPayloads[signedPayload] = notification;
    }

    public AppStoreNotification? Validate(string signedPayload)
    {
        this.validPayloads.TryGetValue(signedPayload, out var notification);
        return notification;
    }
}
