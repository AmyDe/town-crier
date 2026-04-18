using System.Text.Json.Serialization;

namespace TownCrier.Web;

internal sealed record ApiErrorResponse(
    [property: JsonPropertyName("error")] string Error,
    [property: JsonPropertyName("message")] string? Message = null);
