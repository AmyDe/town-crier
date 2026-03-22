using System.Text.Json.Serialization;

namespace TownCrier.Web;

internal sealed record UserIdResponse([property: JsonPropertyName("userId")] string UserId);
