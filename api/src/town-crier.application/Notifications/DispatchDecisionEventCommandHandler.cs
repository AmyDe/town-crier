using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.SavedApplications;
using TownCrier.Application.UserProfiles;
using TownCrier.Application.WatchZones;
using TownCrier.Domain.Notifications;
using TownCrier.Domain.PlanningApplications;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Notifications;

/// <summary>
/// Unified decision-update dispatcher. Computes the union of users matched by
/// a watch zone (geographic) and users who have bookmarked the application
/// (saved). For each candidate user, OR-merges per-channel toggles across the
/// matching sources to decide whether to push and/or queue an email row, then
/// persists one <see cref="Notification"/> with the relevant
/// <see cref="NotificationSources"/> flags. Idempotent — at most one
/// DecisionUpdate notification per (userId, applicationUid).
/// </summary>
public sealed class DispatchDecisionEventCommandHandler
{
    private readonly INotificationRepository notificationRepository;
    private readonly IUserProfileRepository userProfileRepository;
    private readonly ISavedApplicationRepository savedApplicationRepository;
    private readonly IWatchZoneRepository watchZoneRepository;
#pragma warning disable S4487 // Wired in subsequent cycles (push delivery)
    private readonly IDeviceRegistrationRepository deviceRegistrationRepository;
    private readonly IPushNotificationSender pushNotificationSender;
#pragma warning restore S4487
    private readonly TimeProvider timeProvider;

    public DispatchDecisionEventCommandHandler(
        INotificationRepository notificationRepository,
        IUserProfileRepository userProfileRepository,
        ISavedApplicationRepository savedApplicationRepository,
        IWatchZoneRepository watchZoneRepository,
        IDeviceRegistrationRepository deviceRegistrationRepository,
        IPushNotificationSender pushNotificationSender,
        TimeProvider timeProvider)
    {
        this.notificationRepository = notificationRepository;
        this.userProfileRepository = userProfileRepository;
        this.savedApplicationRepository = savedApplicationRepository;
        this.watchZoneRepository = watchZoneRepository;
        this.deviceRegistrationRepository = deviceRegistrationRepository;
        this.pushNotificationSender = pushNotificationSender;
        this.timeProvider = timeProvider;
    }

    public async Task HandleAsync(DispatchDecisionEventCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var application = command.Application;
        var now = this.timeProvider.GetUtcNow();

        // Per-user accumulator: which sources matched, and (for zone matches)
        // the first zoneId we'll attribute the notification to. Saved-only
        // matches leave WatchZoneId null.
        var perUser = new Dictionary<string, (NotificationSources Sources, string? WatchZoneId)>(
            StringComparer.Ordinal);

        // Zone matchers — only meaningful when the application has coordinates.
        if (application.Latitude.HasValue && application.Longitude.HasValue)
        {
            var matchingZones = await this.watchZoneRepository.FindZonesContainingAsync(
                application.Latitude.Value, application.Longitude.Value, ct).ConfigureAwait(false);

            foreach (var zone in matchingZones)
            {
                if (perUser.TryGetValue(zone.UserId, out var existing))
                {
                    perUser[zone.UserId] = (existing.Sources | NotificationSources.Zone, existing.WatchZoneId ?? zone.Id);
                }
                else
                {
                    perUser[zone.UserId] = (NotificationSources.Zone, zone.Id);
                }
            }
        }

        // Saved bookmark holders.
        var savedUserIds = await this.savedApplicationRepository
            .GetUserIdsByApplicationUidAsync(application.Uid, ct)
            .ConfigureAwait(false);

        foreach (var userId in savedUserIds)
        {
            if (perUser.TryGetValue(userId, out var existing))
            {
                perUser[userId] = (existing.Sources | NotificationSources.Saved, existing.WatchZoneId);
            }
            else
            {
                perUser[userId] = (NotificationSources.Saved, null);
            }
        }

        foreach (var (userId, match) in perUser)
        {
            await this.DispatchForUserAsync(userId, match.Sources, match.WatchZoneId, application, now, ct)
                .ConfigureAwait(false);
        }
    }

    private async Task DispatchForUserAsync(
        string userId,
        NotificationSources sources,
        string? watchZoneId,
        PlanningApplication application,
        DateTimeOffset now,
        CancellationToken ct)
    {
        // Idempotency — one DecisionUpdate per (user, applicationUid) ever.
        var existing = await this.notificationRepository.GetByUserAndApplicationAsync(
            userId, application.Uid, NotificationEventType.DecisionUpdate, ct)
            .ConfigureAwait(false);

        if (existing is not null)
        {
            return;
        }

        var profile = await this.userProfileRepository.GetByUserIdAsync(userId, ct)
            .ConfigureAwait(false);

        if (profile is null)
        {
            return;
        }

        var notification = Notification.Create(
            userId: userId,
            applicationUid: application.Uid,
            applicationName: application.Name,
            watchZoneId: watchZoneId,
            applicationAddress: application.Address,
            applicationDescription: application.Description,
            applicationType: application.AppType,
            authorityId: application.AreaId,
            now: now,
            decision: application.AppState,
            eventType: NotificationEventType.DecisionUpdate,
            sources: sources);

        await this.notificationRepository.SaveAsync(notification, ct).ConfigureAwait(false);
    }
}
