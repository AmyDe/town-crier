using TownCrier.Application.Notifications;
using TownCrier.Domain.Notifications;

namespace TownCrier.Application.Tests.Notifications;

internal sealed class SpyEmailSender : IEmailSender
{
    private readonly List<DigestSendRecord> digestsSent = [];
    private readonly List<(string UserId, string Email, Notification Notification)> notificationsSent = [];

    public IReadOnlyList<DigestSendRecord> DigestsSent => this.digestsSent;

    public IReadOnlyList<(string UserId, string Email, Notification Notification)> NotificationsSent => this.notificationsSent;

    public Task SendDigestAsync(
        string userId,
        string email,
        IReadOnlyList<WatchZoneDigest> zoneSections,
        IReadOnlyList<Notification> savedApplications,
        CancellationToken ct)
    {
        this.digestsSent.Add(new DigestSendRecord(userId, email, zoneSections, savedApplications));
        return Task.CompletedTask;
    }

    public Task SendNotificationAsync(string userId, string email, Notification notification, CancellationToken ct)
    {
        this.notificationsSent.Add((userId, email, notification));
        return Task.CompletedTask;
    }
}
