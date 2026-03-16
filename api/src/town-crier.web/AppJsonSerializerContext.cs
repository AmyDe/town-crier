using System.Text.Json.Serialization;
using TownCrier.Application.Geocoding;
using TownCrier.Application.Health;

namespace TownCrier.Web;

[JsonSerializable(typeof(HealthStatus))]
[JsonSerializable(typeof(GeocodePostcodeResult))]
internal sealed partial class AppJsonSerializerContext : JsonSerializerContext;
