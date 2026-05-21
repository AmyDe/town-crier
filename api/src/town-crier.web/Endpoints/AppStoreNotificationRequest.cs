namespace TownCrier.Web.Endpoints;

/// <summary>
/// Request body for <c>POST /v1/webhooks/appstore</c>. <see cref="SignedPayload"/>
/// is the Apple App Store Server Notification v2 signed JWS payload (compact
/// serialization).
/// </summary>
internal sealed record AppStoreNotificationRequest(string SignedPayload);
