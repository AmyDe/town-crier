using TownCrier.Domain.Notifications;

namespace TownCrier.Application.Notifications;

public sealed record WatchZoneDigest(string WatchZoneName, IReadOnlyList<Notification> Notifications);
