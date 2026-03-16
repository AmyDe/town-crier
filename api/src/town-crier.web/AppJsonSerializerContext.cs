using System.Text.Json.Serialization;
using TownCrier.Application.Health;

namespace TownCrier.Web;

[JsonSerializable(typeof(HealthStatus))]
internal sealed partial class AppJsonSerializerContext : JsonSerializerContext;
