using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Cosmos;

internal sealed record CosmosQueryParameter(
    [property: JsonPropertyName("name")] string Name,
    [property: JsonPropertyName("value")] object Value);
