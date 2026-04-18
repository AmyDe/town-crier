using System.Text.Json.Serialization;

namespace Tc.Json;

[JsonSourceGenerationOptions(PropertyNamingPolicy = JsonKnownNamingPolicy.CamelCase)]
[JsonSerializable(typeof(ConfigFile))]
[JsonSerializable(typeof(GenerateOfferCodesRequest))]
[JsonSerializable(typeof(GrantSubscriptionRequest))]
[JsonSerializable(typeof(ListUsersResponse))]
internal sealed partial class TcJsonContext : JsonSerializerContext;
