using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.GovUkPlanningData;

internal sealed class GovUkEntityResponse
{
    [JsonPropertyName("entities")]
    public List<GovUkEntity> Entities { get; set; } = [];
}
