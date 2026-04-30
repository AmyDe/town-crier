using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.UserProfiles;

public sealed class UpdateZonePreferencesCommandHandler
{
    private readonly IUserProfileRepository repository;

    public UpdateZonePreferencesCommandHandler(IUserProfileRepository repository)
    {
        this.repository = repository;
    }

    public async Task<UpdateZonePreferencesResult> HandleAsync(
        UpdateZonePreferencesCommand command,
        CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var profile = await this.repository.GetByUserIdAsync(command.UserId, ct).ConfigureAwait(false)
            ?? throw UserProfileNotFoundException.ForUser(command.UserId);

        var preferences = new ZoneNotificationPreferences(
            command.NewApplicationPush,
            command.NewApplicationEmail,
            command.DecisionPush,
            command.DecisionEmail);

        profile.SetZonePreferences(command.ZoneId, preferences);

        await this.repository.SaveAsync(profile, ct).ConfigureAwait(false);

        return new UpdateZonePreferencesResult(
            command.ZoneId,
            preferences.NewApplicationPush,
            preferences.NewApplicationEmail,
            preferences.DecisionPush,
            preferences.DecisionEmail);
    }
}
