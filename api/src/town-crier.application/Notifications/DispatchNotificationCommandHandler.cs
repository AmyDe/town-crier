using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.Observability;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.Entitlements;
using TownCrier.Domain.Notifications;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Notifications;

public sealed class DispatchNotificationCommandHandler
{
    private const int FreeMonthlyNotificationCap = 5;

    private readonly INotificationRepository notificationRepository;
    private readonly IUserProfileRepository userProfileRepository;
    private readonly IDeviceRegistrationRepository deviceRegistrationRepository;
    private readonly IPushNotificationSender pushNotificationSender;
    private readonly IEmailSender emailSender;
    private readonly TimeProvider timeProvider;

    public DispatchNotificationCommandHandler(
        INotificationRepository notificationRepository,
        IUserProfileRepository userProfileRepository,
        IDeviceRegistrationRepository deviceRegistrationRepository,
        IPushNotificationSender pushNotificationSender,
        IEmailSender emailSender,
        TimeProvider timeProvider)
    {
        this.notificationRepository = notificationRepository;
        this.userProfileRepository = userProfileRepository;
        this.deviceRegistrationRepository = deviceRegistrationRepository;
        this.pushNotificationSender = pushNotificationSender;
        this.emailSender = emailSender;
        this.timeProvider = timeProvider;
    }

    public async Task HandleAsync(DispatchNotificationCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var application = command.Application;
        var zone = command.MatchedZone;
        var now = this.timeProvider.GetUtcNow();

        // Duplicate suppression: same application + same user = one notification
        var existing = await this.notificationRepository.GetByUserAndApplicationAsync(
            zone.UserId, application.Name, ct).ConfigureAwait(false);

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
            applicationName: application.Name,
            watchZoneId: zone.Id,
            applicationAddress: application.Address,
            applicationDescription: application.Description,
            applicationType: application.AppType,
            authorityId: application.AreaId,
            now: now);

        ApiMetrics.NotificationsCreated.Add(1);

        // Check notification preferences — record but don't push
        if (!profile.NotificationPreferences.PushEnabled)
        {
            await this.notificationRepository.SaveAsync(notification, ct).ConfigureAwait(false);
            return;
        }

        // Check zone-level preferences for new applications
        var zonePrefs = profile.GetZonePreferences(zone.Id);
        if (!zonePrefs.NewApplications)
        {
            await this.notificationRepository.SaveAsync(notification, ct).ConfigureAwait(false);
            return;
        }

        // Check free-tier monthly cap
        if (profile.Tier == SubscriptionTier.Free)
        {
            var monthlyCount = await this.notificationRepository.CountByUserInMonthAsync(
                zone.UserId, now.Year, now.Month, ct).ConfigureAwait(false);

            if (monthlyCount >= FreeMonthlyNotificationCap)
            {
                // Cap reached — record notification but don't send push
                await this.notificationRepository.SaveAsync(notification, ct).ConfigureAwait(false);
                return;
            }
        }

        // Send push notification
        var devices = await this.deviceRegistrationRepository.GetByUserIdAsync(zone.UserId, ct)
            .ConfigureAwait(false);

        if (devices.Count > 0)
        {
            await this.pushNotificationSender.SendAsync(notification, devices, ct)
                .ConfigureAwait(false);
            notification.MarkPushSent();
            ApiMetrics.NotificationsSent.Add(1);
        }

        // Send instant email notification for entitled tiers
        var entitlements = EntitlementMap.EntitlementsFor(profile.Tier);
        if (entitlements.Contains(Entitlement.InstantEmails)
            && profile.NotificationPreferences.EmailInstantEnabled
            && !string.IsNullOrEmpty(profile.Email))
        {
            await this.emailSender.SendNotificationAsync(profile.Email, notification, ct)
                .ConfigureAwait(false);
        }

        await this.notificationRepository.SaveAsync(notification, ct).ConfigureAwait(false);
    }
}
