using TownCrier.Application.PlanningApplications;
using TownCrier.Domain.SavedApplications;

namespace TownCrier.Application.SavedApplications;

public sealed class SaveApplicationCommandHandler
{
    private readonly ISavedApplicationRepository repository;
    private readonly IPlanningApplicationRepository planningApplicationRepository;
    private readonly TimeProvider timeProvider;

    public SaveApplicationCommandHandler(
        ISavedApplicationRepository repository,
        IPlanningApplicationRepository planningApplicationRepository,
        TimeProvider timeProvider)
    {
        this.repository = repository;
        this.planningApplicationRepository = planningApplicationRepository;
        this.timeProvider = timeProvider;
    }

    public async Task HandleAsync(SaveApplicationCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        // Persist the full application first so the user/UID record always points
        // at a known planning application. Search no longer upserts, so this is
        // the single write path for ad-hoc applications. See bead tc-if12.
        await this.planningApplicationRepository.UpsertAsync(command.Application, ct).ConfigureAwait(false);

        var alreadySaved = await this.repository.ExistsAsync(command.UserId, command.Application.Uid, ct).ConfigureAwait(false);
        if (alreadySaved)
        {
            return;
        }

        var savedApplication = SavedApplication.Create(command.UserId, command.Application.Uid, this.timeProvider.GetUtcNow());
        await this.repository.SaveAsync(savedApplication, ct).ConfigureAwait(false);
    }
}
