using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Tests.Cosmos;

public sealed class TestDocument
{
    [JsonPropertyName("id")]
    public string Id { get; set; } = string.Empty;

    [JsonPropertyName("name")]
    public string Name { get; set; } = string.Empty;
}

[JsonSerializable(typeof(TestDocument))]
[JsonSerializable(typeof(int))]
internal sealed partial class TestSerializerContext : JsonSerializerContext;
