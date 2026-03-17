using TownCrier.Application.UserProfiles;

namespace TownCrier.Application.Subscriptions;

public sealed class HandleAppStoreNotificationCommandHandler
{
    private readonly IAppStoreNotificationValidator validator;
    private readonly IUserProfileRepository repository;

    public HandleAppStoreNotificationCommandHandler(
        IAppStoreNotificationValidator validator,
        IUserProfileRepository repository)
    {
        this.validator = validator;
        this.repository = repository;
    }

    public Task<HandleAppStoreNotificationResult> HandleAsync(
        HandleAppStoreNotificationCommand command,
        CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var notification = this.validator.Validate(command.SignedPayload);
        if (notification is null)
        {
            return Task.FromResult(new HandleAppStoreNotificationResult(NotificationOutcome.InvalidSignature));
        }

        _ = this.repository;
        return Task.FromResult(new HandleAppStoreNotificationResult(NotificationOutcome.Processed));
    }
}
