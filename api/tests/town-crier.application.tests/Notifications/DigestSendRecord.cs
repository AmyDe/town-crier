using TownCrier.Application.Notifications;
using TownCrier.Domain.Notifications;

namespace TownCrier.Application.Tests.Notifications;

internal sealed record DigestSendRecord(
    string UserId,
    string Email,
    IReadOnlyList<WatchZoneDigest> Digests,
    IReadOnlyList<Notification> SavedApplications);
