using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.GovUkPlanningData;

internal sealed class GovUkEntity
{
    [JsonPropertyName("dataset")]
    public string Dataset { get; set; } = string.Empty;

    [JsonPropertyName("name")]
    public string Name { get; set; } = string.Empty;

    [JsonPropertyName("reference")]
    public string Reference { get; set; } = string.Empty;

    [JsonPropertyName("listed-building-grade")]
    public string? ListedBuildingGrade { get; set; }
}
