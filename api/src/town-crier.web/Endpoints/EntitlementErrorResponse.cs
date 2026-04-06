using System.Text.Json.Serialization;

namespace TownCrier.Web.Endpoints;

internal sealed record EntitlementErrorResponse(
    [property: JsonPropertyName("error")] string Error,
    [property: JsonPropertyName("required")] string Required,
    [property: JsonPropertyName("message")] string Message);
