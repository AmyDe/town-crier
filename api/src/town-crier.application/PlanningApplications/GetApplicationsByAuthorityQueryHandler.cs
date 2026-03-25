namespace TownCrier.Application.PlanningApplications;

public sealed class GetApplicationsByAuthorityQueryHandler
{
    private readonly IPlanningApplicationRepository repository;

    public GetApplicationsByAuthorityQueryHandler(IPlanningApplicationRepository repository)
    {
        this.repository = repository;
    }

    public async Task<IReadOnlyList<PlanningApplicationResult>> HandleAsync(GetApplicationsByAuthorityQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        var applications = await this.repository.GetByAuthorityIdAsync(query.AuthorityId, ct).ConfigureAwait(false);

        return applications.Select(GetApplicationByUidQueryHandler.ToResult).ToList();
    }
}
