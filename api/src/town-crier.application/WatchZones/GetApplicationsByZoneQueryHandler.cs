using System.Globalization;
using TownCrier.Application.Notifications;
using TownCrier.Application.NotificationState;
using TownCrier.Application.PlanningApplications;
using TownCrier.Domain.Notifications;

namespace TownCrier.Application.WatchZones;

public sealed class GetApplicationsByZoneQueryHandler
{
    private static readonly IReadOnlyDictionary<string, Notification> EmptyLatestUnread =
        new Dictionary<string, Notification>(StringComparer.Ordinal);

    private readonly IWatchZoneRepository watchZoneRepository;
    private readonly IPlanningApplicationRepository applicationRepository;
    private readonly INotificationRepository notificationRepository;
    private readonly INotificationStateRepository notificationStateRepository;

    public GetApplicationsByZoneQueryHandler(
        IWatchZoneRepository watchZoneRepository,
        IPlanningApplicationRepository applicationRepository,
        INotificationRepository notificationRepository,
        INotificationStateRepository notificationStateRepository)
    {
        ArgumentNullException.ThrowIfNull(watchZoneRepository);
        ArgumentNullException.ThrowIfNull(applicationRepository);
        ArgumentNullException.ThrowIfNull(notificationRepository);
        ArgumentNullException.ThrowIfNull(notificationStateRepository);

        this.watchZoneRepository = watchZoneRepository;
        this.applicationRepository = applicationRepository;
        this.notificationRepository = notificationRepository;
        this.notificationStateRepository = notificationStateRepository;
    }

    public async Task<IReadOnlyList<PlanningApplicationResult>?> HandleAsync(
        GetApplicationsByZoneQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        var zone = await this.watchZoneRepository.GetByUserAndZoneIdAsync(
            query.UserId, query.ZoneId, ct).ConfigureAwait(false);

        if (zone is null)
        {
            return null;
        }

        var authorityCode = zone.AuthorityId.ToString(CultureInfo.InvariantCulture);
        var applications = await this.applicationRepository.FindNearbyAsync(
            authorityCode,
            zone.Centre.Latitude,
            zone.Centre.Longitude,
            zone.RadiusMetres,
            ct).ConfigureAwait(false);

        // First-touch users have no NotificationState document yet. The
        // applications endpoint is a read-only path; seeding state here would
        // race with the dedicated GET /me/notification-state seeder. So when no
        // watermark exists we surface a null latestUnreadEvent for every row.
        var state = await this.notificationStateRepository
            .GetByUserIdAsync(query.UserId, ct).ConfigureAwait(false);

        // Batch the latest-unread lookup into a single Cosmos round-trip for every
        // application in the zone, rather than one query per application. With ~237
        // apps per zone the former N+1 loop dominated request latency (~6s); the
        // batched query collapses it to O(1) (bd tc-1wkp). When the user has no
        // watermark we can't classify anything as unread, so we skip the lookup.
        IReadOnlyDictionary<string, Notification> latestUnreadByUid =
            EmptyLatestUnread;
        if (state is not null)
        {
            var uids = applications.Select(a => a.Uid).ToArray();
            latestUnreadByUid = await this.notificationRepository
                .GetLatestUnreadByApplicationsAsync(
                    query.UserId, uids, state.LastReadAt, ct)
                .ConfigureAwait(false);
        }

        var results = new List<PlanningApplicationResult>(applications.Count);
        foreach (var application in applications)
        {
            LatestUnreadEvent? latestUnread = null;
            if (latestUnreadByUid.TryGetValue(application.Uid, out var notification))
            {
                latestUnread = new LatestUnreadEvent(
                    notification.EventType,
                    notification.Decision,
                    notification.CreatedAt);
            }

            var row = GetApplicationByUidQueryHandler.ToResult(application) with
            {
                LatestUnreadEvent = latestUnread,
            };
            results.Add(row);
        }

        return results;
    }
}
