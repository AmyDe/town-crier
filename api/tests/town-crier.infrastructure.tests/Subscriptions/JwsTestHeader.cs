using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Tests.Subscriptions;

internal sealed record JwsTestHeader(
    [property: JsonPropertyName("alg")] string Alg,
    [property: JsonPropertyName("x5c")] string[] X5c);
