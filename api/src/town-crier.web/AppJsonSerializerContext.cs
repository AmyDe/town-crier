using System.Text.Json.Serialization;
using TownCrier.Application.Geocoding;
using TownCrier.Application.Health;
using TownCrier.Application.UserProfiles;

namespace TownCrier.Web;

[JsonSerializable(typeof(HealthStatus))]
[JsonSerializable(typeof(GeocodePostcodeResult))]
[JsonSerializable(typeof(CreateUserProfileResult))]
[JsonSerializable(typeof(GetUserProfileResult))]
[JsonSerializable(typeof(UpdateUserProfileCommand))]
[JsonSerializable(typeof(UpdateUserProfileResult))]
internal sealed partial class AppJsonSerializerContext : JsonSerializerContext;
