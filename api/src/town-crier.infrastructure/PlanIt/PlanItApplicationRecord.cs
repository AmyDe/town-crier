using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.PlanIt;

internal sealed class PlanItApplicationRecord
{
    [JsonPropertyName("name")]
    public string Name { get; set; } = string.Empty;

    [JsonPropertyName("uid")]
    public string Uid { get; set; } = string.Empty;

    [JsonPropertyName("area_name")]
    public string AreaName { get; set; } = string.Empty;

    [JsonPropertyName("area_id")]
    public int AreaId { get; set; }

    [JsonPropertyName("address")]
    public string Address { get; set; } = string.Empty;

    [JsonPropertyName("postcode")]
    public string? Postcode { get; set; }

    [JsonPropertyName("description")]
    public string Description { get; set; } = string.Empty;

    [JsonPropertyName("app_type")]
    public string AppType { get; set; } = string.Empty;

    [JsonPropertyName("app_state")]
    public string AppState { get; set; } = string.Empty;

    [JsonPropertyName("app_size")]
    public string? AppSize { get; set; }

    [JsonPropertyName("start_date")]
    public string? StartDate { get; set; }

    [JsonPropertyName("decided_date")]
    public string? DecidedDate { get; set; }

    [JsonPropertyName("consulted_date")]
    public string? ConsultedDate { get; set; }

    [JsonPropertyName("location_x")]
    public double? LocationX { get; set; }

    [JsonPropertyName("location_y")]
    public double? LocationY { get; set; }

    [JsonPropertyName("url")]
    public string? Url { get; set; }

    [JsonPropertyName("link")]
    public string? Link { get; set; }

    [JsonPropertyName("last_different")]
    public string LastDifferent { get; set; } = string.Empty;
}
