using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Authorities;

internal sealed class AuthorityRecord
{
    [JsonPropertyName("id")]
    public int Id { get; set; }

    [JsonPropertyName("name")]
    public string Name { get; set; } = string.Empty;

    [JsonPropertyName("areaType")]
    public string AreaType { get; set; } = string.Empty;
}
