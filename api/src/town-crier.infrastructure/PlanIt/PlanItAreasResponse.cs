using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.PlanIt;

internal sealed class PlanItAreasResponse
{
    [JsonPropertyName("records")]
    public List<PlanItAreaRecord> Records { get; set; } = [];

    [JsonPropertyName("total")]
    public int Total { get; set; }
}
