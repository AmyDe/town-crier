using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Tests.Subscriptions;

[JsonSourceGenerationOptions(WriteIndented = false)]
[JsonSerializable(typeof(JwsTestHeader))]
internal sealed partial class TestJwsJsonContext : JsonSerializerContext;
