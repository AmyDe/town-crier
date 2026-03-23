using System.Text.Json.Serialization;
using TownCrier.Domain.Geocoding;
using TownCrier.Domain.UserProfiles;
using TownCrier.Infrastructure.SavedApplications;

namespace TownCrier.Infrastructure.Cosmos;

[JsonSerializable(typeof(Coordinates))]
[JsonSerializable(typeof(NotificationPreferences))]
[JsonSerializable(typeof(ZoneNotificationPreferences))]
[JsonSerializable(typeof(SavedApplicationDocument))]
[JsonSerializable(typeof(List<SavedApplicationDocument>))]
internal sealed partial class CosmosJsonSerializerContext : JsonSerializerContext;
