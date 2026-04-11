using TownCrier.Application.Subscriptions;

namespace TownCrier.Application.Tests.Subscriptions;

internal sealed class FakeNotificationDecoder : INotificationDecoder
{
    private readonly Dictionary<string, DecodedNotification> notifications = [];

    public void Register(string json, DecodedNotification notification)
    {
        this.notifications[json] = notification;
    }

    public DecodedNotification Decode(string json)
    {
        if (this.notifications.TryGetValue(json, out var notification))
        {
            return notification;
        }

        throw new InvalidOperationException($"No registered notification for JSON: '{json}'");
    }
}
