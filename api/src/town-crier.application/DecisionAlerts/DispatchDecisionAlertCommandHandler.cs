using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.SavedApplications;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.DecisionAlerts;

namespace TownCrier.Application.DecisionAlerts;

public sealed class DispatchDecisionAlertCommandHandler
{
    private readonly IDecisionAlertRepository alertRepository;
    private readonly ISavedApplicationRepository savedApplicationRepository;
    private readonly IUserProfileRepository userProfileRepository;
    private readonly IDeviceRegistrationRepository deviceRegistrationRepository;
    private readonly IDecisionAlertPushSender pushSender;
    private readonly TimeProvider timeProvider;

    public DispatchDecisionAlertCommandHandler(
        IDecisionAlertRepository alertRepository,
        ISavedApplicationRepository savedApplicationRepository,
        IUserProfileRepository userProfileRepository,
        IDeviceRegistrationRepository deviceRegistrationRepository,
        IDecisionAlertPushSender pushSender,
        TimeProvider timeProvider)
    {
        this.alertRepository = alertRepository;
        this.savedApplicationRepository = savedApplicationRepository;
        this.userProfileRepository = userProfileRepository;
        this.deviceRegistrationRepository = deviceRegistrationRepository;
        this.pushSender = pushSender;
        this.timeProvider = timeProvider;
    }

    public async Task HandleAsync(DispatchDecisionAlertCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var application = command.Application;
        var now = this.timeProvider.GetUtcNow();

        // Find all users who bookmarked this application
        var userIds = await this.savedApplicationRepository.GetUserIdsByApplicationUidAsync(
            application.Uid, ct).ConfigureAwait(false);

        foreach (var userId in userIds)
        {
            await this.DispatchForUserAsync(userId, application, now, ct).ConfigureAwait(false);
        }
    }

    private async Task DispatchForUserAsync(
        string userId,
        Domain.PlanningApplications.PlanningApplication application,
        DateTimeOffset now,
        CancellationToken ct)
    {
        // Duplicate suppression: one alert per user per application
        var existing = await this.alertRepository.GetByUserAndApplicationAsync(
            userId, application.Uid, ct).ConfigureAwait(false);

        if (existing is not null)
        {
            return;
        }

        // Load user profile to check preferences
        var profile = await this.userProfileRepository.GetByUserIdAsync(userId, ct)
            .ConfigureAwait(false);

        if (profile is null)
        {
            return;
        }

        // Create decision alert record
        var alert = DecisionAlert.Create(
            userId: userId,
            applicationUid: application.Uid,
            applicationName: application.Name,
            applicationAddress: application.Address,
            decision: application.AppState ?? "Unknown",
            now: now);

        // Check push preferences
        if (!profile.NotificationPreferences.PushEnabled)
        {
            await this.alertRepository.SaveAsync(alert, ct).ConfigureAwait(false);
            return;
        }

        // Send push notification
        var devices = await this.deviceRegistrationRepository.GetByUserIdAsync(userId, ct)
            .ConfigureAwait(false);

        if (devices.Count > 0)
        {
            await this.pushSender.SendAsync(alert, devices, ct).ConfigureAwait(false);
            alert.MarkPushSent();
        }

        await this.alertRepository.SaveAsync(alert, ct).ConfigureAwait(false);
    }
}
