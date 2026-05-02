namespace TownCrier.Application.Notifications;

/// <summary>
/// Outcome of a push send attempt. <see cref="InvalidTokens"/> lists device
/// tokens that APNs rejected as permanently invalid (e.g. 410 Unregistered or
/// 400 BadDeviceToken). The handler is responsible for pruning these from the
/// device-registration store; the sender only reports them.
/// </summary>
public sealed record PushSendResult(IReadOnlyList<string> InvalidTokens)
{
    public static PushSendResult Empty { get; } = new(Array.Empty<string>());
}
