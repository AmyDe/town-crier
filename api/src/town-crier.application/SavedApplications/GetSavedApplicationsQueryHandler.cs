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

        var results = new List<SavedApplicationResult>();
        foreach (var s in saved)
        {
            var application = await this.applicationRepository.GetByUidAsync(s.ApplicationUid, ct).ConfigureAwait(false);
            if (application is not null)
            {
                results.Add(new SavedApplicationResult(
                    s.ApplicationUid,
                    s.SavedAt,
                    GetApplicationByUidQueryHandler.ToResult(application)));
            }
        }

        return results;
    }
}
