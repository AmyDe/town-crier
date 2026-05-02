using TownCrier.Application.Notifications;
using TownCrier.Domain.DeviceRegistrations;
using TownCrier.Domain.Notifications;

namespace TownCrier.Application.Tests.Notifications;

internal sealed class SpyPushNotificationSender : IPushNotificationSender
{
    private readonly List<(Notification Notification, IReadOnlyList<DeviceRegistration> Devices)> sent = [];
    private readonly List<(int ApplicationCount, IReadOnlyList<DeviceRegistration> Devices)> digestsSent = [];

    public IReadOnlyList<(Notification Notification, IReadOnlyList<DeviceRegistration> Devices)> Sent => this.sent;

    public IReadOnlyList<(int ApplicationCount, IReadOnlyList<DeviceRegistration> Devices)> DigestsSent => this.digestsSent;

    /// <summary>
    /// Tokens to surface in <see cref="PushSendResult.InvalidTokens"/> on the
    /// next <see cref="SendAsync"/> call. Mirrors the real APNs sender's 410
    /// Unregistered / 400 BadDeviceToken signal so handlers can be exercised
    /// against the prune path. Defaults to empty (no rejections).
    /// </summary>
    public IReadOnlyList<string> NextInvalidTokens { get; set; } = Array.Empty<string>();

    /// <summary>
    /// Tokens to surface in <see cref="PushSendResult.InvalidTokens"/> on the
    /// next <see cref="SendDigestAsync"/> call.
    /// </summary>
    public IReadOnlyList<string> NextInvalidDigestTokens { get; set; } = Array.Empty<string>();

    public Task<PushSendResult> SendAsync(Notification notification, IReadOnlyList<DeviceRegistration> devices, CancellationToken ct)
    {
        this.sent.Add((notification, devices));
        var result = this.NextInvalidTokens.Count == 0
            ? PushSendResult.Empty
            : new PushSendResult(this.NextInvalidTokens);
        return Task.FromResult(result);
    }

    public Task<PushSendResult> SendDigestAsync(int applicationCount, IReadOnlyList<DeviceRegistration> devices, CancellationToken ct)
    {
        this.digestsSent.Add((applicationCount, devices));
        var result = this.NextInvalidDigestTokens.Count == 0
            ? PushSendResult.Empty
            : new PushSendResult(this.NextInvalidDigestTokens);
        return Task.FromResult(result);
    }
}
