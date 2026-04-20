using TownCrier.Application.Auth;
using TownCrier.Application.DecisionAlerts;
using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.Notifications;
using TownCrier.Application.SavedApplications;
using TownCrier.Application.WatchZones;

namespace TownCrier.Application.UserProfiles;

public sealed class DeleteUserProfileCommandHandler
{
    private readonly IUserProfileRepository repository;
    private readonly IAuth0ManagementClient auth0Client;
    private readonly INotificationRepository notificationRepository;
    private readonly IDecisionAlertRepository decisionAlertRepository;
    private readonly IWatchZoneRepository watchZoneRepository;
    private readonly ISavedApplicationRepository savedApplicationRepository;
    private readonly IDeviceRegistrationRepository deviceRegistrationRepository;

    public DeleteUserProfileCommandHandler(
        IUserProfileRepository repository,
        IAuth0ManagementClient auth0Client,
        INotificationRepository notificationRepository,
        IDecisionAlertRepository decisionAlertRepository,
        IWatchZoneRepository watchZoneRepository,
        ISavedApplicationRepository savedApplicationRepository,
        IDeviceRegistrationRepository deviceRegistrationRepository)
    {
        this.repository = repository;
        this.auth0Client = auth0Client;
        this.notificationRepository = notificationRepository;
        this.decisionAlertRepository = decisionAlertRepository;
        this.watchZoneRepository = watchZoneRepository;
        this.savedApplicationRepository = savedApplicationRepository;
        this.deviceRegistrationRepository = deviceRegistrationRepository;
    }

    public async Task HandleAsync(DeleteUserProfileCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var profile = await this.repository.GetByUserIdAsync(command.UserId, ct).ConfigureAwait(false);
        if (profile is null)
        {
            throw UserProfileNotFoundException.ForUser(command.UserId);
        }

        // Cascade-delete child records before removing the profile so that, if
        // a cascade step fails, the profile still exists and the caller can retry.
        // The privacy policy promises removal of notifications, decision alerts,
        // watch zones, saved applications, and device registrations.
        await this.notificationRepository.DeleteAllByUserIdAsync(command.UserId, ct).ConfigureAwait(false);
        await this.decisionAlertRepository.DeleteAllByUserIdAsync(command.UserId, ct).ConfigureAwait(false);
        await this.watchZoneRepository.DeleteAllByUserIdAsync(command.UserId, ct).ConfigureAwait(false);
        await this.savedApplicationRepository.DeleteAllByUserIdAsync(command.UserId, ct).ConfigureAwait(false);
        await this.deviceRegistrationRepository.DeleteAllByUserIdAsync(command.UserId, ct).ConfigureAwait(false);

        await this.repository.DeleteAsync(command.UserId, ct).ConfigureAwait(false);
        await this.auth0Client.DeleteUserAsync(command.UserId, ct).ConfigureAwait(false);
    }
}
