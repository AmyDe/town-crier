using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.UserProfiles;

public sealed class UpdateUserProfileCommandHandler
{
    private readonly IUserProfileRepository repository;

    public UpdateUserProfileCommandHandler(IUserProfileRepository repository)
    {
        this.repository = repository;
    }

    public async Task<UpdateUserProfileResult> HandleAsync(UpdateUserProfileCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var profile = await this.repository.GetByUserIdAsync(command.UserId, ct).ConfigureAwait(false)
            ?? throw UserProfileNotFoundException.ForUser(command.UserId);

        profile.UpdatePreferences(new NotificationPreferences(
            command.PushEnabled,
            command.DigestDay,
            command.EmailDigestEnabled,
            command.SavedDecisionPush,
            command.SavedDecisionEmail));
        await this.repository.SaveAsync(profile, ct).ConfigureAwait(false);

        return new UpdateUserProfileResult(
            profile.UserId,
            profile.NotificationPreferences.PushEnabled,
            profile.NotificationPreferences.DigestDay,
            profile.NotificationPreferences.EmailDigestEnabled,
            profile.NotificationPreferences.SavedDecisionPush,
            profile.NotificationPreferences.SavedDecisionEmail,
            profile.Tier);
    }
}
