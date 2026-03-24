using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.PlanningApplications;

public sealed class GetApplicationByUidQueryHandler
{
    private readonly IPlanningApplicationRepository repository;

    public GetApplicationByUidQueryHandler(IPlanningApplicationRepository repository)
    {
        this.repository = repository;
    }

    public async Task<PlanningApplicationResult?> HandleAsync(GetApplicationByUidQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        var application = await this.repository.GetByUidAsync(query.Uid, ct).ConfigureAwait(false);
        if (application is null)
        {
            return null;
        }

        return ToResult(application);
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
