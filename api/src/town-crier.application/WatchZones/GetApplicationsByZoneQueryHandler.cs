using System.Globalization;
using TownCrier.Application.Notifications;
using TownCrier.Application.NotificationState;
using TownCrier.Application.PlanningApplications;

namespace TownCrier.Application.WatchZones;

public sealed class GetApplicationsByZoneQueryHandler
{
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

        var results = new List<PlanningApplicationResult>(applications.Count);
        foreach (var application in applications)
        {
            LatestUnreadEvent? latestUnread = null;
            if (state is not null)
            {
                var notification = await this.notificationRepository
                    .GetLatestUnreadByApplicationAsync(
                        query.UserId, application.Uid, state.LastReadAt, ct)
                    .ConfigureAwait(false);

                if (notification is not null)
                {
                    latestUnread = new LatestUnreadEvent(
                        notification.EventType,
                        notification.Decision,
                        notification.CreatedAt);
                }
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
