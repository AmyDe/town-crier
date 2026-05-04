using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.NotificationState;
using TownCrier.Application.Observability;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.Notifications;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Notifications;

public sealed class DispatchNotificationCommandHandler
{
    private readonly INotificationRepository notificationRepository;
    private readonly INotificationStateRepository notificationStateRepository;
    private readonly IUserProfileRepository userProfileRepository;
    private readonly IDeviceRegistrationRepository deviceRegistrationRepository;
    private readonly IPushNotificationSender pushNotificationSender;
    private readonly RemoveInvalidDeviceTokenCommandHandler removeInvalidDeviceTokenHandler;
    private readonly TimeProvider timeProvider;

    public DispatchNotificationCommandHandler(
        INotificationRepository notificationRepository,
        INotificationStateRepository notificationStateRepository,
        IUserProfileRepository userProfileRepository,
        IDeviceRegistrationRepository deviceRegistrationRepository,
        IPushNotificationSender pushNotificationSender,
        RemoveInvalidDeviceTokenCommandHandler removeInvalidDeviceTokenHandler,
        TimeProvider timeProvider)
    {
        this.notificationRepository = notificationRepository;
        this.notificationStateRepository = notificationStateRepository;
        this.userProfileRepository = userProfileRepository;
        this.deviceRegistrationRepository = deviceRegistrationRepository;
        this.pushNotificationSender = pushNotificationSender;
        this.removeInvalidDeviceTokenHandler = removeInvalidDeviceTokenHandler;
        this.timeProvider = timeProvider;
    }

    public async Task HandleAsync(DispatchNotificationCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var application = command.Application;
        var zone = command.MatchedZone;
        var now = this.timeProvider.GetUtcNow();

        // Duplicate suppression: dedup by (userId, applicationUid, eventType).
        // This handler always emits NewApplication; DecisionUpdate dispatch lives
        // on a separate path so the same user can receive one of each per app.
        var existing = await this.notificationRepository.GetByUserAndApplicationAsync(
            zone.UserId,
            application.Uid,
            NotificationEventType.NewApplication,
            ct).ConfigureAwait(false);

        if (existing is not null)
        {
            return;
        }

        // Load user profile to check preferences and tier
        var profile = await this.userProfileRepository.GetByUserIdAsync(zone.UserId, ct)
            .ConfigureAwait(false);

        if (profile is null)
        {
            return;
        }

        // Create notification record
        var notification = Notification.Create(
            userId: zone.UserId,
            applicationUid: application.Uid,
            applicationName: application.Name,
            watchZoneId: zone.Id,
            applicationAddress: application.Address,
            applicationDescription: application.Description,
            applicationType: application.AppType,
            authorityId: application.AreaId,
            now: now);

        ApiMetrics.NotificationsCreated.Add(
            1,
            new KeyValuePair<string, object?>("event_type", notification.EventType.ToString()),
            new KeyValuePair<string, object?>("sources", notification.Sources.ToString()));

        // Check notification preferences — record but don't push
        if (!profile.NotificationPreferences.PushEnabled)
        {
            await this.notificationRepository.SaveAsync(notification, ct).ConfigureAwait(false);
            return;
        }

        // Per-zone push toggle (T2 — WatchZone.PushEnabled). Independent of the
        // user-profile-level zone preferences, this is the toggle the user
        // controls in the WatchZone editor on iOS/web.
        if (!zone.PushEnabled)
        {
            await this.notificationRepository.SaveAsync(notification, ct).ConfigureAwait(false);
            return;
        }

        // Check zone-level preferences for new applications
        var zonePrefs = profile.GetZonePreferences(zone.Id);
        if (!zonePrefs.NewApplicationPush)
        {
            await this.notificationRepository.SaveAsync(notification, ct).ConfigureAwait(false);
            return;
        }

        // Paid-tier gate: instant push is a paid-tier hook. Free tier still gets the
        // notification row (picked up by the weekly digest) but no push.
        if (profile.Tier == SubscriptionTier.Free)
        {
            await this.notificationRepository.SaveAsync(notification, ct).ConfigureAwait(false);
            return;
        }

        // Send push notification
        var devices = await this.deviceRegistrationRepository.GetByUserIdAsync(zone.UserId, ct)
            .ConfigureAwait(false);

        if (devices.Count > 0)
        {
            // Compute the badge value from the watermark — count of unread
            // notifications (createdAt > lastReadAt). The newly-created
            // notification isn't persisted yet, but is unread by construction
            // (now > lastReadAt for any sane watermark), so we add 1 for it.
            // First-touch users have no notification-state document; we treat
            // their watermark as DateTimeOffset.MinValue, making everything
            // unread and the badge accurate from the very first push.
            var totalUnreadCount = await this.ComputeTotalUnreadCountAsync(zone.UserId, ct)
                .ConfigureAwait(false) + 1;

            var sendResult = await this.pushNotificationSender
                .SendAsync(notification, devices, totalUnreadCount, ct)
                .ConfigureAwait(false);
            notification.MarkPushSent();
            ApiMetrics.NotificationsSent.Add(
                1,
                new KeyValuePair<string, object?>("event_type", notification.EventType.ToString()),
                new KeyValuePair<string, object?>("sources", notification.Sources.ToString()),
                new KeyValuePair<string, object?>("tier", profile.Tier.ToString()));

            // Prune device tokens APNs reported as permanently invalid (e.g. 410
            // Unregistered, 400 BadDeviceToken). Idempotent — empty InvalidTokens
            // dispatches no commands.
            foreach (var invalidToken in sendResult.InvalidTokens)
            {
                await this.removeInvalidDeviceTokenHandler
                    .HandleAsync(new RemoveInvalidDeviceTokenCommand(invalidToken), ct)
                    .ConfigureAwait(false);
            }
        }

        await this.notificationRepository.SaveAsync(notification, ct).ConfigureAwait(false);
    }

    private async Task<int> ComputeTotalUnreadCountAsync(string userId, CancellationToken ct)
    {
        // First-touch users may have no notification-state row yet; treat the
        // watermark as DateTimeOffset.MinValue so every persisted notification
        // counts as unread. The endpoint adapter seeds a real document on first
        // GET — until then we assume nothing has been read.
        var state = await this.notificationStateRepository
            .GetByUserIdAsync(userId, ct).ConfigureAwait(false);
        var lastReadAt = state?.LastReadAt ?? DateTimeOffset.MinValue;
        return await this.notificationRepository
            .GetUnreadCountAsync(userId, lastReadAt, ct).ConfigureAwait(false);
    }
}
