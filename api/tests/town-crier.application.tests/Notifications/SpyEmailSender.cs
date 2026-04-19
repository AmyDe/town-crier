using TownCrier.Application.Notifications;
using TownCrier.Domain.Notifications;

namespace TownCrier.Application.Tests.Notifications;

internal sealed class SpyEmailSender : IEmailSender
{
    private readonly List<(string UserId, string Email, IReadOnlyList<WatchZoneDigest> Digests)> digestsSent = [];
    private readonly List<(string UserId, string Email, Notification Notification)> notificationsSent = [];

    public IReadOnlyList<(string UserId, string Email, IReadOnlyList<WatchZoneDigest> Digests)> DigestsSent => this.digestsSent;

    public IReadOnlyList<(string UserId, string Email, Notification Notification)> NotificationsSent => this.notificationsSent;

    public Task SendDigestAsync(string userId, string email, IReadOnlyList<WatchZoneDigest> digests, CancellationToken ct)
    {
        this.digestsSent.Add((userId, email, digests));
        return Task.CompletedTask;
    }

    public Task SendNotificationAsync(string userId, string email, Notification notification, CancellationToken ct)
    {
        this.notificationsSent.Add((userId, email, notification));
        return Task.CompletedTask;
    }
}
