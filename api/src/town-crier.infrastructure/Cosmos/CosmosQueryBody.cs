using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Cosmos;

internal sealed record CosmosQueryBody(
    [property: JsonPropertyName("query")] string Query,
    [property: JsonPropertyName("parameters")] IReadOnlyList<CosmosQueryParameter>? Parameters);
