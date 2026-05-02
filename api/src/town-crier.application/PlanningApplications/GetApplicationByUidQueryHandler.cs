using TownCrier.Application.PlanIt;
using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.PlanningApplications;

public sealed class GetApplicationByUidQueryHandler
{
    private readonly IPlanningApplicationRepository repository;
    private readonly IPlanItClient planItClient;

    public GetApplicationByUidQueryHandler(
        IPlanningApplicationRepository repository,
        IPlanItClient planItClient)
    {
        this.repository = repository;
        this.planItClient = planItClient;
    }

    public async Task<PlanningApplicationResult?> HandleAsync(GetApplicationByUidQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        var application = await this.repository.GetByUidAsync(query.Uid, ct).ConfigureAwait(false);
        if (application is not null)
        {
            return ToResult(application);
        }

        // Cosmos miss: fall back to PlanIt's per-application endpoint. This
        // closes the search→tap→details gap left when SearchPlanningApplications
        // stopped upserting search results (see tc-if12).
        var fetched = await this.planItClient.GetByUidAsync(query.Uid, ct).ConfigureAwait(false);
        if (fetched is null)
        {
            return null;
        }

        await this.repository.UpsertAsync(fetched, ct).ConfigureAwait(false);
        return ToResult(fetched);
    }

    internal static PlanningApplicationResult ToResult(PlanningApplication application)
    {
        return new PlanningApplicationResult(
            application.Name,
            application.Uid,
            application.AreaName,
            application.AreaId,
            application.Address,
            application.Postcode,
            application.Description,
            application.AppType,
            application.AppState,
            application.AppSize,
            application.StartDate,
            application.DecidedDate,
            application.ConsultedDate,
            application.Longitude,
            application.Latitude,
            application.Url,
            application.Link,
            application.LastDifferent);
    }
}
