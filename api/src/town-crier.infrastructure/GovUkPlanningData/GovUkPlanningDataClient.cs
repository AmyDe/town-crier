#pragma warning disable CA1062
using System.Globalization;
using System.Net;
using System.Text.Json;
using TownCrier.Application.Designations;
using TownCrier.Domain.Designations;

namespace TownCrier.Infrastructure.GovUkPlanningData;

public sealed class GovUkPlanningDataClient : IDesignationDataProvider
{
    private const string Datasets = "conservation-area,listed-building-outline,article-4-direction-area";

    private readonly HttpClient httpClient;

    public GovUkPlanningDataClient(HttpClient httpClient)
    {
        this.httpClient = httpClient;
    }

    public async Task<DesignationContext> GetDesignationsAsync(
        double latitude,
        double longitude,
        CancellationToken ct)
    {
        var point = string.Create(
            CultureInfo.InvariantCulture,
            $"POINT({longitude} {latitude})");
        var encodedPoint = Uri.EscapeDataString(point);
        var url = new Uri(
            $"/api/v1/entity.json?geometry_intersects={encodedPoint}&dataset={Datasets}",
            UriKind.Relative);

        using var response = await this.httpClient
            .GetAsync(url, ct)
            .ConfigureAwait(false);

        // The entity endpoint returns 404 when the query geometry doesn't
        // intersect any entity in the requested datasets — most UK points
        // aren't inside a conservation area, listed building, or article-4 area.
        if (response.StatusCode == HttpStatusCode.NotFound)
        {
            return DesignationContext.None;
        }

        response.EnsureSuccessStatusCode();

        var entityResponse = await JsonSerializer.DeserializeAsync(
            await response.Content.ReadAsStreamAsync(ct).ConfigureAwait(false),
            GovUkPlanningDataJsonSerializerContext.Default.GovUkEntityResponse,
            ct).ConfigureAwait(false);

        if (entityResponse is null || entityResponse.Entities.Count == 0)
        {
            return DesignationContext.None;
        }

        return MapToDesignationContext(entityResponse.Entities);
    }

    private static DesignationContext MapToDesignationContext(List<GovUkEntity> entities)
    {
        var conservationArea = entities.Find(e =>
            string.Equals(e.Dataset, "conservation-area", StringComparison.OrdinalIgnoreCase));

        var listedBuilding = entities.Find(e =>
            string.Equals(e.Dataset, "listed-building-outline", StringComparison.OrdinalIgnoreCase));

        var article4 = entities.Find(e =>
            string.Equals(e.Dataset, "article-4-direction-area", StringComparison.OrdinalIgnoreCase));

        return new DesignationContext(
            isWithinConservationArea: conservationArea is not null,
            conservationAreaName: conservationArea?.Name,
            isWithinListedBuildingCurtilage: listedBuilding is not null,
            listedBuildingGrade: listedBuilding?.ListedBuildingGrade,
            isWithinArticle4Area: article4 is not null);
    }
}
