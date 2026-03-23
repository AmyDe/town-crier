using System.Text.Json.Serialization;
using TownCrier.Domain.Geocoding;
using TownCrier.Domain.UserProfiles;
using TownCrier.Infrastructure.PlanningApplications;

namespace TownCrier.Infrastructure.Cosmos;

[JsonSerializable(typeof(Coordinates))]
[JsonSerializable(typeof(NotificationPreferences))]
[JsonSerializable(typeof(ZoneNotificationPreferences))]
[JsonSerializable(typeof(PlanningApplicationDocument))]
[JsonSerializable(typeof(List<PlanningApplicationDocument>))]
[JsonSerializable(typeof(GeoJsonPoint))]
internal sealed partial class CosmosJsonSerializerContext : JsonSerializerContext;
