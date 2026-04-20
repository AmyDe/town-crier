using TownCrier.Application.Auth;

namespace TownCrier.Application.UserProfiles;

public sealed class DeleteUserProfileCommandHandler
{
    private readonly IUserProfileRepository repository;
    private readonly IAuth0ManagementClient auth0Client;

    public DeleteUserProfileCommandHandler(
        IUserProfileRepository repository,
        IAuth0ManagementClient auth0Client)
    {
        this.repository = repository;
        this.auth0Client = auth0Client;
    }

    public async Task HandleAsync(DeleteUserProfileCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var profile = await this.repository.GetByUserIdAsync(command.UserId, ct).ConfigureAwait(false);
        if (profile is null)
        {
            throw UserProfileNotFoundException.ForUser(command.UserId);
        }

        await this.repository.DeleteAsync(command.UserId, ct).ConfigureAwait(false);
        await this.auth0Client.DeleteUserAsync(command.UserId, ct).ConfigureAwait(false);
    }
}
