using TownCrier.Application.DecisionAlerts;
using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.Notifications;
using TownCrier.Application.OfferCodes;
using TownCrier.Application.SavedApplications;
using TownCrier.Application.WatchZones;

namespace TownCrier.Application.UserProfiles;

public sealed class ExportUserDataQueryHandler
{
    private readonly IUserProfileRepository userProfileRepository;
    private readonly IWatchZoneRepository watchZoneRepository;
    private readonly INotificationRepository notificationRepository;
    private readonly IDecisionAlertRepository decisionAlertRepository;
    private readonly ISavedApplicationRepository savedApplicationRepository;
    private readonly IDeviceRegistrationRepository deviceRegistrationRepository;
    private readonly IOfferCodeRepository offerCodeRepository;

    public ExportUserDataQueryHandler(
        IUserProfileRepository userProfileRepository,
        IWatchZoneRepository watchZoneRepository,
        INotificationRepository notificationRepository,
        IDecisionAlertRepository decisionAlertRepository,
        ISavedApplicationRepository savedApplicationRepository,
        IDeviceRegistrationRepository deviceRegistrationRepository,
        IOfferCodeRepository offerCodeRepository)
    {
        this.userProfileRepository = userProfileRepository;
        this.watchZoneRepository = watchZoneRepository;
        this.notificationRepository = notificationRepository;
        this.decisionAlertRepository = decisionAlertRepository;
        this.savedApplicationRepository = savedApplicationRepository;
        this.deviceRegistrationRepository = deviceRegistrationRepository;
        this.offerCodeRepository = offerCodeRepository;
    }

    public async Task<ExportUserDataResult?> HandleAsync(ExportUserDataQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        var profile = await this.userProfileRepository.GetByUserIdAsync(query.UserId, ct).ConfigureAwait(false);
        if (profile is null)
        {
            return null;
        }

        var watchZones = await this.watchZoneRepository.GetByUserIdAsync(query.UserId, ct).ConfigureAwait(false);
        var notifications = await this.notificationRepository.GetByUserSinceAsync(
            query.UserId, DateTimeOffset.MinValue, ct).ConfigureAwait(false);
        var decisionAlerts = await this.decisionAlertRepository.GetByUserIdAsync(query.UserId, ct).ConfigureAwait(false);
        var savedApplications = await this.savedApplicationRepository.GetByUserIdAsync(query.UserId, ct).ConfigureAwait(false);
        var deviceRegistrations = await this.deviceRegistrationRepository.GetByUserIdAsync(query.UserId, ct).ConfigureAwait(false);
        var offerCodes = await this.offerCodeRepository.GetRedeemedByUserIdAsync(query.UserId, ct).ConfigureAwait(false);

        return new ExportUserDataResult(
            UserId: profile.UserId,
            Email: profile.Email,
            NotificationPreferences: new ExportedNotificationPreferences(
                PushEnabled: profile.NotificationPreferences.PushEnabled,
                DigestDay: profile.NotificationPreferences.DigestDay,
                EmailDigestEnabled: profile.NotificationPreferences.EmailDigestEnabled,
                ZonePreferences: profile.AllZonePreferences
                    .Select(kvp => new ExportedZonePreferences(
                        ZoneId: kvp.Key,
                        NewApplicationPush: kvp.Value.NewApplicationPush,
                        NewApplicationEmail: kvp.Value.NewApplicationEmail,
                        DecisionPush: kvp.Value.DecisionPush,
                        DecisionEmail: kvp.Value.DecisionEmail))
                    .ToList()),
            Subscription: new ExportedSubscription(
                Tier: profile.Tier,
                ExpiresAt: profile.SubscriptionExpiry,
                OriginalTransactionId: profile.OriginalTransactionId,
                GracePeriodExpiresAt: profile.GracePeriodExpiry),
            WatchZones: watchZones
                .Select(z => new ExportedWatchZone(
                    Id: z.Id,
                    Name: z.Name,
                    Latitude: z.Centre.Latitude,
                    Longitude: z.Centre.Longitude,
                    RadiusMetres: z.RadiusMetres,
                    AuthorityId: z.AuthorityId,
                    CreatedAt: z.CreatedAt))
                .ToList(),
            Notifications: notifications
                .Select(n => new ExportedNotification(
                    Id: n.Id,
                    ApplicationName: n.ApplicationName,
                    WatchZoneId: n.WatchZoneId,
                    ApplicationAddress: n.ApplicationAddress,
                    ApplicationDescription: n.ApplicationDescription,
                    ApplicationType: n.ApplicationType,
                    AuthorityId: n.AuthorityId,
                    PushSent: n.PushSent,
                    EmailSent: n.EmailSent,
                    CreatedAt: n.CreatedAt))
                .ToList(),
            DecisionAlerts: decisionAlerts
                .Select(a => new ExportedDecisionAlert(
                    Id: a.Id,
                    ApplicationUid: a.ApplicationUid,
                    ApplicationName: a.ApplicationName,
                    ApplicationAddress: a.ApplicationAddress,
                    Decision: a.Decision,
                    PushSent: a.PushSent,
                    CreatedAt: a.CreatedAt))
                .ToList(),
            SavedApplications: savedApplications
                .Select(s => new ExportedSavedApplication(
                    ApplicationUid: s.ApplicationUid,
                    SavedAt: s.SavedAt))
                .ToList(),
            DeviceRegistrations: deviceRegistrations
                .Select(d => new ExportedDeviceRegistration(
                    Token: d.Token,
                    Platform: d.Platform,
                    RegisteredAt: d.RegisteredAt))
                .ToList(),
            OfferCodeRedemptions: offerCodes
                .Select(c => new ExportedOfferCodeRedemption(
                    Code: c.Code,
                    Tier: c.Tier,
                    DurationDays: c.DurationDays,
                    RedeemedAt: c.RedeemedAt ?? DateTimeOffset.MinValue))
                .ToList());
    }
}
