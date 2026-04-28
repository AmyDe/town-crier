using TownCrier.Application.PlanningApplications;

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

        var saved = await this.savedRepository.GetByUserIdAsync(query.UserId, ct).ConfigureAwait(false);

        // Hydrate every save concurrently. The previous implementation issued N sequential
        // GetByUidAsync calls, which dominated cold-load latency for users with several saves
        // (see bd tc-qz0j / tc-a1x8). Order is preserved by hydrating into a positional array.
        var hydrated = await Task.WhenAll(
            saved.Select(s => this.applicationRepository.GetByUidAsync(s.ApplicationUid, ct))).ConfigureAwait(false);

        var results = new List<SavedApplicationResult>(saved.Count);
        for (var i = 0; i < saved.Count; i++)
        {
            var application = hydrated[i];
            if (application is not null)
            {
                results.Add(new SavedApplicationResult(
                    saved[i].ApplicationUid,
                    saved[i].SavedAt,
                    GetApplicationByUidQueryHandler.ToResult(application)));
            }
        }

        return results;
    }
}
