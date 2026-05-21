using System.Text.Json;
using TownCrier.Application.Subscriptions;

namespace TownCrier.Infrastructure.Subscriptions;

/// <summary>
/// Maps the raw JSON payload of a verified Apple App Store Server Notification
/// v2 onto the application-layer <see cref="DecodedNotification"/>. Native
/// AOT-safe — uses System.Text.Json source generation only.
/// </summary>
public sealed class NotificationDecoder : INotificationDecoder
{
    public DecodedNotification Decode(string json)
    {
        if (string.IsNullOrWhiteSpace(json))
        {
            throw new ArgumentException("The notification JSON is empty.", nameof(json));
        }

        AppleNotificationPayload? payload;
        try
        {
            payload = JsonSerializer.Deserialize(
                json, SubscriptionsJsonSerializerContext.Default.AppleNotificationPayload);
        }
        catch (JsonException ex)
        {
            throw new ArgumentException("The notification JSON is malformed.", nameof(json), ex);
        }

        if (payload is null)
        {
            throw new ArgumentException("The notification JSON is null.", nameof(json));
        }

        return new DecodedNotification(
            NotificationType: Require(payload.NotificationType, "notificationType"),
            Subtype: payload.Subtype,
            NotificationUuid: Require(payload.NotificationUuid, "notificationUUID"),
            SignedTransactionInfo: Require(
                payload.Data?.SignedTransactionInfo, "data.signedTransactionInfo"),
            SignedRenewalInfo: payload.Data?.SignedRenewalInfo);
    }

    private static string Require(string? value, string field) =>
        string.IsNullOrEmpty(value)
            ? throw new ArgumentException(
                $"The notification JSON is missing the required '{field}' field.")
            : value;
}
