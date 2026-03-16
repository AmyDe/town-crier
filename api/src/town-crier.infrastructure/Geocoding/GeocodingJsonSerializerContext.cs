using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Geocoding;

[JsonSerializable(typeof(PostcodesIoResponse))]
internal sealed partial class GeocodingJsonSerializerContext : JsonSerializerContext;
