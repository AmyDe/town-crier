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

        // Idempotency keys on the canonical {areaId}/{name} uid, NOT the raw uid the
        // client put in the request body. The raw uid format varies between clients
        // (PR #398 changed it), so trusting it would let a re-save with a stale-format
        // uid miss the existing row and insert a second saved doc (bd tc-o88i).
        var canonicalUid = command.Application.CanonicalUid;
        var alreadySaved = await this.repository.ExistsAsync(command.UserId, canonicalUid, ct).ConfigureAwait(false);
        if (alreadySaved)
        {
            return;
        }

        // Embed the full snapshot so the saved-list endpoint renders with one
        // partitioned query and never fans out to N cross-partition reads. See
        // bd tc-udby for the 429 storm this design eliminates.
        var savedApplication = SavedApplication.Create(command.UserId, command.Application, this.timeProvider.GetUtcNow());
        await this.repository.SaveAsync(savedApplication, ct).ConfigureAwait(false);
    }
}
