using System.Text.Json.Serialization;
using TownCrier.Domain.Geocoding;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Infrastructure.Cosmos;

[JsonSerializable(typeof(Coordinates))]
[JsonSerializable(typeof(NotificationPreferences))]
[JsonSerializable(typeof(ZoneNotificationPreferences))]
internal sealed partial class CosmosJsonSerializerContext : JsonSerializerContext;
