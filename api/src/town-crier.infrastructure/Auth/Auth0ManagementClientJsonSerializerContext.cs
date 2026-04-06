using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Auth;

[JsonSerializable(typeof(Auth0TokenResponse))]
[JsonSerializable(typeof(Auth0TokenRequest))]
[JsonSerializable(typeof(Auth0UpdateMetadataRequest))]
internal sealed partial class Auth0ManagementClientJsonSerializerContext : JsonSerializerContext;
