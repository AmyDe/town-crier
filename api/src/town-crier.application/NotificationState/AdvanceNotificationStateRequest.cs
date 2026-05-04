namespace TownCrier.Application.NotificationState;

/// <summary>
/// Wire shape for <c>POST /v1/me/notification-state/advance</c>. The user id
/// is taken from the authenticated principal so it is not part of the body —
/// only the candidate <c>asOf</c> instant (typically the tapped notification's
/// <c>CreatedAt</c>) is supplied. See spec Pre-Resolved Decision #11.
/// </summary>
/// <param name="AsOf">The candidate new watermark.</param>
public sealed record AdvanceNotificationStateRequest(DateTimeOffset AsOf);
