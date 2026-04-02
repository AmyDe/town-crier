using TownCrier.Application.PlanIt;
using TownCrier.Application.PlanningApplications;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Search;

public sealed class SearchPlanningApplicationsQueryHandler
{
    private readonly IUserProfileRepository userProfileRepository;
    private readonly IPlanItClient planItClient;
    private readonly IPlanningApplicationRepository applicationRepository;

    public SearchPlanningApplicationsQueryHandler(
        IUserProfileRepository userProfileRepository,
        IPlanItClient planItClient,
        IPlanningApplicationRepository applicationRepository)
    {
        this.userProfileRepository = userProfileRepository;
        this.planItClient = planItClient;
        this.applicationRepository = applicationRepository;
    }

    public async Task<SearchPlanningApplicationsResult> HandleAsync(
        SearchPlanningApplicationsQuery query,
        CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        var profile = await this.userProfileRepository.GetByUserIdAsync(query.UserId, ct).ConfigureAwait(false)
            ?? throw UserProfileNotFoundException.ForUser(query.UserId);

        if (profile.Tier != SubscriptionTier.Pro)
        {
            throw new ProTierRequiredException();
        }

        var result = await this.planItClient.SearchApplicationsAsync(
            query.SearchText, query.AuthorityId, query.Page, ct).ConfigureAwait(false);

        foreach (var application in result.Applications)
        {
            await this.applicationRepository.UpsertAsync(application, ct).ConfigureAwait(false);
        }

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

        return new SearchPlanningApplicationsResult(summaries, result.Total, query.Page);
    }
}
