using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.UserProfiles;

public sealed class CreateUserProfileCommandHandler
{
    private readonly IUserProfileRepository repository;

    public CreateUserProfileCommandHandler(IUserProfileRepository repository)
    {
        this.repository = repository;
    }

    public async Task<CreateUserProfileResult> HandleAsync(CreateUserProfileCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var existing = await this.repository.GetByUserIdAsync(command.UserId, ct).ConfigureAwait(false);
        if (existing is not null)
        {
            return new CreateUserProfileResult(
                existing.UserId,
                existing.Postcode,
                existing.NotificationPreferences.PushEnabled,
                existing.Tier);
        }

        var profile = UserProfile.Register(command.UserId, command.Email);
        await this.repository.SaveAsync(profile, ct).ConfigureAwait(false);

        return new CreateUserProfileResult(
            profile.UserId,
            profile.Postcode,
            profile.NotificationPreferences.PushEnabled,
            profile.Tier);
    }
}
