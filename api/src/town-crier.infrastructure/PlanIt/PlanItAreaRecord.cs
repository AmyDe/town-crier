using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.PlanIt;

internal sealed class PlanItAreaRecord
{
    [JsonPropertyName("area_name")]
    public string Name { get; set; } = string.Empty;

    [JsonPropertyName("area_id")]
    public int Id { get; set; }

    [JsonPropertyName("area_type")]
    public string AreaType { get; set; } = string.Empty;

    [JsonPropertyName("url")]
    public string? CouncilUrl { get; set; }

    [JsonPropertyName("planning_url")]
    public string? PlanningUrl { get; set; }
}
