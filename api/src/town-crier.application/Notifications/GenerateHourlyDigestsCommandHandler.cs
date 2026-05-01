using TownCrier.Application.Observability;
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

            // Per-zone instant-email gating (T2 — WatchZone.EmailInstantEnabled).
            // Notifications attached to a zone with EmailInstantEnabled=false are
            // excluded from this hourly digest (and left unsent so the weekly
            // digest can still pick them up — weekly is unaffected by per-zone
            // flags). Saved-only notifications (WatchZoneId null) bypass the
            // per-zone gate; they're driven by the saved bookmark contract.
            var instantEnabledZones = zones
                .Where(z => z.EmailInstantEnabled)
                .Select(z => z.Id)
                .ToHashSet(StringComparer.Ordinal);

            var includedNotifications = notifications
                .Where(n => n.WatchZoneId is null || instantEnabledZones.Contains(n.WatchZoneId))
                .ToList();

            if (includedNotifications.Count == 0)
            {
                continue;
            }

            var digests = includedNotifications
                .Where(n => n.WatchZoneId is not null)
                .GroupBy(n => n.WatchZoneId!)
                .Select(g => new WatchZoneDigest(
                    zoneLookup.GetValueOrDefault(g.Key, "Unknown Zone"),
                    g.ToList()))
                .ToList();

            var savedApplications = includedNotifications
                .Where(n => n.WatchZoneId is null)
                .ToList();

            await this.emailSender
                .SendDigestAsync(profile.UserId, profile.Email, digests, savedApplications, ct)
                .ConfigureAwait(false);

            foreach (var notification in includedNotifications)
            {
                ApiMetrics.DigestRowsEmitted.Add(
                    1,
                    new KeyValuePair<string, object?>("cadence", "hourly"),
                    new KeyValuePair<string, object?>("event_type", notification.EventType.ToString()));

                notification.MarkEmailSent();
                await this.notificationRepository.SaveAsync(notification, ct)
                    .ConfigureAwait(false);
            }
        }
    }
}
