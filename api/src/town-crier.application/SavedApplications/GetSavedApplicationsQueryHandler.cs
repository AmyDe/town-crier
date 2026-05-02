using TownCrier.Application.PlanningApplications;
using TownCrier.Domain.SavedApplications;

namespace TownCrier.Application.SavedApplications;

public sealed class GetSavedApplicationsQueryHandler
{
    private readonly ISavedApplicationRepository savedRepository;
    private readonly IPlanningApplicationRepository applicationRepository;

    public GetSavedApplicationsQueryHandler(
        ISavedApplicationRepository savedRepository,
        IPlanningApplicationRepository applicationRepository)
    {
        this.savedRepository = savedRepository;
        this.applicationRepository = applicationRepository;
    }

    public async Task<IReadOnlyList<SavedApplicationResult>> HandleAsync(GetSavedApplicationsQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        // One partitioned query — Cosmos returns the saved rows together with their
        // embedded planning-application snapshot. No N-fan-out cross-partition
        // hydration. See bd tc-udby for the 429 storm this design eliminates.
        var saved = await this.savedRepository.GetByUserIdAsync(query.UserId, ct).ConfigureAwait(false);

        var results = new List<SavedApplicationResult>(saved.Count);
        foreach (var record in saved)
        {
            var snapshot = record.Application;
            if (snapshot is null)
            {
                // Lazy backfill: rows persisted before the snapshot column existed
                // hold only the uid. Hydrate once and upsert so subsequent reads
                // are zero-hydration. Self-heals on first read per legacy row.
                var fetched = await this.applicationRepository.GetByUidAsync(record.ApplicationUid, ct).ConfigureAwait(false);
                if (fetched is null)
                {
                    // Master record gone — exclude rather than failing the whole list.
                    continue;
                }

                var refreshed = SavedApplication.Create(record.UserId, fetched, record.SavedAt);
                await this.savedRepository.SaveAsync(refreshed, ct).ConfigureAwait(false);
                snapshot = fetched;
            }

            results.Add(new SavedApplicationResult(
                record.ApplicationUid,
                record.SavedAt,
                GetApplicationByUidQueryHandler.ToResult(snapshot)));
        }

        return results;
    }
}
