namespace TownCrier.Application.NotificationState;

/// <summary>
/// Advances the caller's watermark to a client-supplied instant (the tapped
/// notification's <c>CreatedAt</c>). Monotonic — instants at or before the
/// current watermark are silently ignored. See spec Pre-Resolved Decision #11.
/// </summary>
/// <param name="UserId">The Auth0 sub of the caller.</param>
/// <param name="AsOf">The candidate new watermark (typically the tapped notification's <c>CreatedAt</c>).</param>
public sealed record AdvanceNotificationStateCommand(string UserId, DateTimeOffset AsOf);
