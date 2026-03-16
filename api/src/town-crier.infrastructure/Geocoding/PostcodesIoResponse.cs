using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Geocoding;

internal sealed class PostcodesIoResponse
{
    [JsonPropertyName("status")]
    public int Status { get; set; }

    [JsonPropertyName("result")]
    public PostcodesIoResult? Result { get; set; }
}
