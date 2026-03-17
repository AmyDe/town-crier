using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.PlanIt;

internal sealed class PlanItAreaRecord
{
    [JsonPropertyName("name")]
    public string Name { get; set; } = string.Empty;

    [JsonPropertyName("id")]
    public int Id { get; set; }

    [JsonPropertyName("area_type")]
    public string AreaType { get; set; } = string.Empty;

    [JsonPropertyName("council_url")]
    public string? CouncilUrl { get; set; }

    [JsonPropertyName("planning_url")]
    public string? PlanningUrl { get; set; }
}
