namespace TownCrier.Application.UserProfiles;

public sealed class DeleteUserProfileCommandHandler
{
    private readonly IUserProfileRepository repository;

    public DeleteUserProfileCommandHandler(IUserProfileRepository repository)
    {
        this.repository = repository;
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
    }
}
