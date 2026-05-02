using TownCrier.Application.Observability;
using TownCrier.Application.PlanIt;
using TownCrier.Application.UserProfiles;

namespace TownCrier.Application.Search;

public sealed class SearchPlanningApplicationsQueryHandler
{
    private readonly IUserProfileRepository userProfileRepository;
    private readonly IPlanItClient planItClient;

    public SearchPlanningApplicationsQueryHandler(
        IUserProfileRepository userProfileRepository,
        IPlanItClient planItClient)
    {
        this.userProfileRepository = userProfileRepository;
        this.planItClient = planItClient;
    }

    public async Task<SearchPlanningApplicationsResult> HandleAsync(
        SearchPlanningApplicationsQuery query,
        CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        _ = await this.userProfileRepository.GetByUserIdAsync(query.UserId, ct).ConfigureAwait(false)
            ?? throw UserProfileNotFoundException.ForUser(query.UserId);

        var result = await this.planItClient.SearchApplicationsAsync(
            query.SearchText, query.AuthorityId, query.Page, ct).ConfigureAwait(false);

        // Do NOT upsert search results into Cosmos. The previous per-application
        // upsert loop was the dominant source of the user-facing 429 burst.
        // Apps are upserted lazily on save (SaveApplicationCommandHandler) and
        // on detail-page Cosmos miss (GetApplicationByUidQueryHandler).
        // See bead tc-if12.
        var summaries = result.Applications.Select(a => new PlanningApplicationSummary(
            a.Uid,
            a.Name,
            a.Address,
            a.Postcode,
            a.Description,
            a.AppType,
            a.AppState,
            a.AreaName,
            a.StartDate,
            a.Url)).ToList();

        ApiMetrics.SearchesPerformed.Add(1);
        return new SearchPlanningApplicationsResult(summaries, result.Total, query.Page);
    }
}
