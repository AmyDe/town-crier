using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.Observability;
using TownCrier.Application.UserProfiles;
using TownCrier.Application.WatchZones;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Notifications;

public sealed class GenerateWeeklyDigestsCommandHandler
{
    private readonly IUserProfileRepository userProfileRepository;
    private readonly INotificationRepository notificationRepository;
    private readonly IDeviceRegistrationRepository deviceRegistrationRepository;
    private readonly IPushNotificationSender pushNotificationSender;
    private readonly IEmailSender emailSender;
    private readonly IWatchZoneRepository watchZoneRepository;
    private readonly TimeProvider timeProvider;

    public GenerateWeeklyDigestsCommandHandler(
        IUserProfileRepository userProfileRepository,
        INotificationRepository notificationRepository,
        IDeviceRegistrationRepository deviceRegistrationRepository,
        IPushNotificationSender pushNotificationSender,
        IEmailSender emailSender,
        IWatchZoneRepository watchZoneRepository,
        TimeProvider timeProvider)
    {
        this.userProfileRepository = userProfileRepository;
        this.notificationRepository = notificationRepository;
        this.deviceRegistrationRepository = deviceRegistrationRepository;
        this.pushNotificationSender = pushNotificationSender;
        this.emailSender = emailSender;
        this.watchZoneRepository = watchZoneRepository;
        this.timeProvider = timeProvider;
    }

    public async Task HandleAsync(GenerateWeeklyDigestsCommand command, CancellationToken ct)
    {
        var now = this.timeProvider.GetUtcNow();
        var today = now.DayOfWeek;
        var since = now.AddDays(-7);

        var users = await this.userProfileRepository.GetAllByDigestDayAsync(today, ct)
            .ConfigureAwait(false);

        foreach (var profile in users)
        {
            var wantsPush = profile.Tier == SubscriptionTier.Pro
                && profile.NotificationPreferences.PushEnabled;
            var wantsEmail = profile.NotificationPreferences.EmailDigestEnabled
                && !string.IsNullOrEmpty(profile.Email);

            if (!wantsPush && !wantsEmail)
            {
                continue;
            }

            var notifications = await this.notificationRepository.GetByUserSinceAsync(
                profile.UserId, since, ct).ConfigureAwait(false);

            if (notifications.Count == 0)
            {
                continue;
            }

            foreach (var notification in notifications)
            {
                ApiMetrics.DigestRowsEmitted.Add(
                    1,
                    new KeyValuePair<string, object?>("cadence", "weekly"),
                    new KeyValuePair<string, object?>("event_type", notification.EventType.ToString()));
            }

            if (wantsPush)
            {
                var devices = await this.deviceRegistrationRepository.GetByUserIdAsync(profile.UserId, ct)
                    .ConfigureAwait(false);

                if (devices.Count > 0)
                {
                    await this.pushNotificationSender.SendDigestAsync(notifications.Count, devices, ct)
                        .ConfigureAwait(false);
                }
            }

            if (wantsEmail)
            {
                var zones = await this.watchZoneRepository.GetByUserIdAsync(profile.UserId, ct)
                    .ConfigureAwait(false);

                var zoneLookup = zones.ToDictionary(z => z.Id, z => z.Name);

                var digests = notifications
                    .Where(n => n.WatchZoneId is not null)
                    .GroupBy(n => n.WatchZoneId!)
                    .Select(g => new WatchZoneDigest(
                        zoneLookup.GetValueOrDefault(g.Key, "Unknown Zone"),
                        g.ToList()))
                    .ToList();

                var savedApplications = notifications
                    .Where(n => n.WatchZoneId is null)
                    .ToList();

                await this.emailSender
                    .SendDigestAsync(profile.UserId, profile.Email!, digests, savedApplications, ct)
                    .ConfigureAwait(false);
            }
        }
    }
}
