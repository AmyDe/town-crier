using System.Text.Json.Serialization;

namespace Tc.Json;

[JsonSourceGenerationOptions(PropertyNamingPolicy = JsonKnownNamingPolicy.CamelCase)]
[JsonSerializable(typeof(ConfigFile))]
[JsonSerializable(typeof(GrantSubscriptionRequest))]
[JsonSerializable(typeof(ListUsersResponse))]
[JsonSerializable(typeof(ListUsersItemResponse))]
internal sealed partial class TcJsonContext : JsonSerializerContext;
