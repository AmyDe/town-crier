using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Geocoding;

[JsonSerializable(typeof(PostcodesIoResponse))]
[JsonSerializable(typeof(PostcodesIoReverseResponse))]
internal sealed partial class GeocodingJsonSerializerContext : JsonSerializerContext;
