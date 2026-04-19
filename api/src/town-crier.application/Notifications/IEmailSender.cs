using TownCrier.Domain.Notifications;

namespace TownCrier.Application.Notifications;

public interface IEmailSender
{
    Task SendDigestAsync(string userId, string email, IReadOnlyList<WatchZoneDigest> digests, CancellationToken ct);

    Task SendNotificationAsync(string userId, string email, Notification notification, CancellationToken ct);
}
