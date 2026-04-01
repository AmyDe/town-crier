using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Geocoding;

[JsonSerializable(typeof(PostcodesIoResponse))]
[JsonSerializable(typeof(PostcodesIoReverseResponse))]
[JsonSerializable(typeof(Dictionary<string, int>))]
internal sealed partial class GeocodingJsonSerializerContext : JsonSerializerContext;
