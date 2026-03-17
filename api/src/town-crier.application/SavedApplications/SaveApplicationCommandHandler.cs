using TownCrier.Domain.SavedApplications;

namespace TownCrier.Application.SavedApplications;

public sealed class SaveApplicationCommandHandler
{
    private readonly ISavedApplicationRepository repository;
    private readonly TimeProvider timeProvider;

    public SaveApplicationCommandHandler(ISavedApplicationRepository repository, TimeProvider timeProvider)
    {
        this.repository = repository;
        this.timeProvider = timeProvider;
    }

    public async Task HandleAsync(SaveApplicationCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var alreadySaved = await this.repository.ExistsAsync(command.UserId, command.ApplicationUid, ct).ConfigureAwait(false);
        if (alreadySaved)
        {
            return;
        }

        var savedApplication = SavedApplication.Create(command.UserId, command.ApplicationUid, this.timeProvider.GetUtcNow());
        await this.repository.SaveAsync(savedApplication, ct).ConfigureAwait(false);
    }
}
