using System.Text.Json.Serialization;

namespace TownCrier.Web.Observability;

[JsonSerializable(typeof(ErrorResponse))]
internal sealed partial class ObservabilityJsonSerializerContext : JsonSerializerContext;
