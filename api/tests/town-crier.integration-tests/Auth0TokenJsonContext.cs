using System.Text.Json.Serialization;

namespace TownCrier.IntegrationTests;

[JsonSerializable(typeof(Auth0TokenProvider.TokenResponse))]
internal sealed partial class Auth0TokenJsonContext : JsonSerializerContext;
