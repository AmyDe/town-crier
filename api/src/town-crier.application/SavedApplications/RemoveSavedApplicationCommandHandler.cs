namespace TownCrier.Application.SavedApplications;

public sealed class RemoveSavedApplicationCommandHandler
{
    private readonly ISavedApplicationRepository repository;

    public RemoveSavedApplicationCommandHandler(ISavedApplicationRepository repository)
    {
        this.repository = repository;
    }

    public async Task HandleAsync(RemoveSavedApplicationCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        await this.repository.DeleteAsync(command.UserId, command.ApplicationUid, ct).ConfigureAwait(false);
    }
}
