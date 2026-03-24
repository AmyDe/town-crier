using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Geocoding;

internal sealed class PostcodesIoReverseResponse
{
    [JsonPropertyName("status")]
    public int Status { get; set; }

    [JsonPropertyName("result")]
    public List<PostcodesIoResult>? Result { get; set; }
}
