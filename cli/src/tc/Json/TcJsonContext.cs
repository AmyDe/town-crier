using System.Text.Json.Serialization;

namespace Tc.Json;

[JsonSerializable(typeof(ConfigFile))]
[JsonSerializable(typeof(GrantSubscriptionRequest))]
internal sealed partial class TcJsonContext : JsonSerializerContext;
