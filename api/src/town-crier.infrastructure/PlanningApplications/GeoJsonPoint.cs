namespace TownCrier.Infrastructure.PlanningApplications;

internal sealed class GeoJsonPoint
{
    public string Type { get; init; } = "Point";

    public double[] Coordinates { get; init; } = [];
}
