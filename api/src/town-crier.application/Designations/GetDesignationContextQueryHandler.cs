#pragma warning disable CA1062
using TownCrier.Domain.Designations;

namespace TownCrier.Application.Designations;

public sealed class GetDesignationContextQueryHandler
{
    private readonly IDesignationDataProvider designationDataProvider;

    public GetDesignationContextQueryHandler(IDesignationDataProvider designationDataProvider)
    {
        this.designationDataProvider = designationDataProvider;
    }

    public async Task<GetDesignationContextResult> HandleAsync(
        GetDesignationContextQuery query,
        CancellationToken ct)
    {
        DesignationContext context;

        try
        {
            context = await this.designationDataProvider
                .GetDesignationsAsync(query.Latitude, query.Longitude, ct)
                .ConfigureAwait(false);
        }
        catch (HttpRequestException)
        {
            context = DesignationContext.None;
        }

        return new GetDesignationContextResult(
            context.IsWithinConservationArea,
            context.ConservationAreaName,
            context.IsWithinListedBuildingCurtilage,
            context.ListedBuildingGrade,
            context.IsWithinArticle4Area);
    }
}
