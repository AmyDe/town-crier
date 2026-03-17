using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Notifications;

public sealed class GenerateWeeklyDigestsCommandHandler
{
    private readonly IUserProfileRepository userProfileRepository;
    private readonly INotificationRepository notificationRepository;
    private readonly IDeviceRegistrationRepository deviceRegistrationRepository;
    private readonly IPushNotificationSender pushNotificationSender;
    private readonly TimeProvider timeProvider;

    public GenerateWeeklyDigestsCommandHandler(
        IUserProfileRepository userProfileRepository,
        INotificationRepository notificationRepository,
        IDeviceRegistrationRepository deviceRegistrationRepository,
        IPushNotificationSender pushNotificationSender,
        TimeProvider timeProvider)
    {
        this.userProfileRepository = userProfileRepository;
        this.notificationRepository = notificationRepository;
        this.deviceRegistrationRepository = deviceRegistrationRepository;
        this.pushNotificationSender = pushNotificationSender;
        this.timeProvider = timeProvider;
    }

    public async Task HandleAsync(GenerateWeeklyDigestsCommand command, CancellationToken ct)
    {
        var now = this.timeProvider.GetUtcNow();
        var today = now.DayOfWeek;
        var since = now.AddDays(-7);

        var proUsers = await this.userProfileRepository.GetAllByTierAsync(SubscriptionTier.Pro, ct)
            .ConfigureAwait(false);

        foreach (var profile in proUsers)
        {
            if (profile.NotificationPreferences.DigestDay != today)
            {
                continue;
            }

            if (!profile.NotificationPreferences.PushEnabled)
            {
                continue;
            }

            var applicationCount = await this.notificationRepository.CountByUserSinceAsync(
                profile.UserId, since, ct).ConfigureAwait(false);

            if (applicationCount == 0)
            {
                continue;
            }

            var devices = await this.deviceRegistrationRepository.GetByUserIdAsync(profile.UserId, ct)
                .ConfigureAwait(false);

            if (devices.Count > 0)
            {
                await this.pushNotificationSender.SendDigestAsync(applicationCount, devices, ct)
                    .ConfigureAwait(false);
            }
        }
    }
}
