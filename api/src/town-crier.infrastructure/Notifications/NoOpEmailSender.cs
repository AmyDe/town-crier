using TownCrier.Application.Notifications;
using TownCrier.Domain.Notifications;

namespace TownCrier.Infrastructure.Notifications;

public sealed class NoOpEmailSender : IEmailSender
{
    public Task SendDigestAsync(string email, IReadOnlyList<WatchZoneDigest> digests, CancellationToken ct)
    {
        return Task.CompletedTask;
    }

    public Task SendNotificationAsync(string email, Notification notification, CancellationToken ct)
    {
        return Task.CompletedTask;
    }
}
