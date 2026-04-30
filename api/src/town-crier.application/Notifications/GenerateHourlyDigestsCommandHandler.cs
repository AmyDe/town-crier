using TownCrier.Application.UserProfiles;
using TownCrier.Application.WatchZones;
using TownCrier.Domain.Entitlements;

namespace TownCrier.Application.Notifications;

public sealed class GenerateHourlyDigestsCommandHandler
{
    private readonly INotificationRepository notificationRepository;
    private readonly IUserProfileRepository userProfileRepository;
    private readonly IEmailSender emailSender;
    private readonly IWatchZoneRepository watchZoneRepository;

    public GenerateHourlyDigestsCommandHandler(
        INotificationRepository notificationRepository,
        IUserProfileRepository userProfileRepository,
        IEmailSender emailSender,
        IWatchZoneRepository watchZoneRepository)
    {
        this.notificationRepository = notificationRepository;
        this.userProfileRepository = userProfileRepository;
        this.emailSender = emailSender;
        this.watchZoneRepository = watchZoneRepository;
    }

    public async Task HandleAsync(GenerateHourlyDigestsCommand command, CancellationToken ct)
    {
        var userIds = await this.notificationRepository.GetUserIdsWithUnsentEmailsAsync(ct)
            .ConfigureAwait(false);

        foreach (var userId in userIds)
        {
            var profile = await this.userProfileRepository.GetByUserIdAsync(userId, ct)
                .ConfigureAwait(false);

            if (profile is null)
            {
                continue;
            }

            var entitlements = EntitlementMap.EntitlementsFor(profile.Tier);
            if (!entitlements.Contains(Entitlement.HourlyDigestEmails))
            {
                continue;
            }

            if (string.IsNullOrEmpty(profile.Email))
            {
                continue;
            }

            if (!profile.NotificationPreferences.EmailDigestEnabled)
            {
                continue;
            }

            var notifications = await this.notificationRepository.GetUnsentEmailsByUserAsync(userId, ct)
                .ConfigureAwait(false);

            if (notifications.Count == 0)
            {
                continue;
            }

            var zones = await this.watchZoneRepository.GetByUserIdAsync(userId, ct)
                .ConfigureAwait(false);

            var zoneLookup = zones.ToDictionary(z => z.Id, z => z.Name);

            var digests = notifications
                .GroupBy(n => n.WatchZoneId ?? string.Empty)
                .Select(g => new WatchZoneDigest(
                    zoneLookup.GetValueOrDefault(g.Key, "Unknown Zone"),
                    g.ToList()))
                .ToList();

            await this.emailSender.SendDigestAsync(profile.UserId, profile.Email, digests, ct)
                .ConfigureAwait(false);

            foreach (var notification in notifications)
            {
                notification.MarkEmailSent();
                await this.notificationRepository.SaveAsync(notification, ct)
                    .ConfigureAwait(false);
            }
        }
    }
}
