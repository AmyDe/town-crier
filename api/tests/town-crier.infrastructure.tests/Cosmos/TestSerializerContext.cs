using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Tests.Cosmos;

[JsonSerializable(typeof(TestDocument))]
[JsonSerializable(typeof(int))]
internal sealed partial class TestSerializerContext : JsonSerializerContext;
