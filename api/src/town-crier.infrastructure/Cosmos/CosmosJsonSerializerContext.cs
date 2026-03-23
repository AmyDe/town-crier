using System.Text.Json.Serialization;
using TownCrier.Domain.Geocoding;
using TownCrier.Domain.UserProfiles;
using TownCrier.Infrastructure.UserProfiles;

namespace TownCrier.Infrastructure.Cosmos;

[JsonSerializable(typeof(Coordinates))]
[JsonSerializable(typeof(NotificationPreferences))]
[JsonSerializable(typeof(ZoneNotificationPreferences))]
[JsonSerializable(typeof(UserProfileDocument))]
[JsonSerializable(typeof(List<UserProfileDocument>))]
internal sealed partial class CosmosJsonSerializerContext : JsonSerializerContext;
