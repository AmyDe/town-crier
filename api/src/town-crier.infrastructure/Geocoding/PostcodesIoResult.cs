using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Geocoding;

internal sealed class PostcodesIoResult
{
    [JsonPropertyName("postcode")]
    public string Postcode { get; set; } = string.Empty;

    [JsonPropertyName("latitude")]
    public double Latitude { get; set; }

    [JsonPropertyName("longitude")]
    public double Longitude { get; set; }
}
