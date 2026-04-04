using TownCrier.Domain.Notifications;

namespace TownCrier.Application.Notifications;

public interface IEmailSender
{
    Task SendDigestAsync(string email, IReadOnlyList<WatchZoneDigest> digests, CancellationToken ct);

    Task SendNotificationAsync(string email, Notification notification, CancellationToken ct);
}
