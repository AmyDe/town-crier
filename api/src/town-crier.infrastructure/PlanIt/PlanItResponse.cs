using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.PlanIt;

internal sealed class PlanItResponse
{
    [JsonPropertyName("records")]
    public List<PlanItApplicationRecord> Records { get; set; } = [];

    [JsonPropertyName("pg_sz")]
    public int PageSize { get; set; }

    [JsonPropertyName("from")]
    public int From { get; set; }

    [JsonPropertyName("total")]
    public int Total { get; set; }
}
