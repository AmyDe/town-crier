using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Subscriptions;

/// <summary>
/// The decoded protected header of an Apple JWS. <c>alg</c> is the signature
/// algorithm (always <c>ES256</c> for App Store payloads); <c>x5c</c> is the
/// base64-DER certificate chain, leaf first.
/// </summary>
internal sealed record JwsHeader(
    [property: JsonPropertyName("alg")] string? Alg,
    [property: JsonPropertyName("x5c")] string[]? X5c);
