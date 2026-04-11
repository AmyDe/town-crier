namespace TownCrier.Application.Subscriptions;

/// <summary>
/// Decodes a raw JSON payload (from a verified JWS) into a <see cref="DecodedNotification"/>.
/// </summary>
public interface INotificationDecoder
{
    DecodedNotification Decode(string json);
}
