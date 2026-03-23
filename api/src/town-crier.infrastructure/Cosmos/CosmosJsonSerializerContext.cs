using System.Text.Json.Serialization;
using TownCrier.Domain.Geocoding;
using TownCrier.Domain.UserProfiles;
using TownCrier.Infrastructure.Notifications;

namespace TownCrier.Infrastructure.Cosmos;

[JsonSerializable(typeof(Coordinates))]
[JsonSerializable(typeof(NotificationPreferences))]
[JsonSerializable(typeof(ZoneNotificationPreferences))]
[JsonSerializable(typeof(NotificationDocument))]
[JsonSerializable(typeof(List<NotificationDocument>))]
internal sealed partial class CosmosJsonSerializerContext : JsonSerializerContext;
