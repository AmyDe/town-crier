using System.Text.Json.Serialization;

namespace Tc.Json;

internal sealed class ConfigFile
{
    [JsonPropertyName("url")]
    public string? Url { get; set; }

    [JsonPropertyName("apiKey")]
    public string? ApiKey { get; set; }
}
