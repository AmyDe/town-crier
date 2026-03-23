using System.Text.Json.Serialization;
using TownCrier.Domain.Geocoding;
using TownCrier.Domain.UserProfiles;
using TownCrier.Infrastructure.DeviceRegistrations;
using TownCrier.Infrastructure.Notifications;
using TownCrier.Infrastructure.PlanningApplications;
using TownCrier.Infrastructure.SavedApplications;
using TownCrier.Infrastructure.WatchZones;

namespace TownCrier.Infrastructure.Cosmos;

[JsonSerializable(typeof(Coordinates))]
[JsonSerializable(typeof(NotificationPreferences))]
[JsonSerializable(typeof(ZoneNotificationPreferences))]
[JsonSerializable(typeof(DeviceRegistrationDocument))]
[JsonSerializable(typeof(List<DeviceRegistrationDocument>))]
[JsonSerializable(typeof(SavedApplicationDocument))]
[JsonSerializable(typeof(List<SavedApplicationDocument>))]
[JsonSerializable(typeof(WatchZoneDocument))]
[JsonSerializable(typeof(List<WatchZoneDocument>))]
[JsonSerializable(typeof(AuthorityZoneCountResult))]
[JsonSerializable(typeof(int))]
[JsonSerializable(typeof(NotificationDocument))]
[JsonSerializable(typeof(List<NotificationDocument>))]
[JsonSerializable(typeof(PlanningApplicationDocument))]
[JsonSerializable(typeof(List<PlanningApplicationDocument>))]
[JsonSerializable(typeof(GeoJsonPoint))]
internal sealed partial class CosmosJsonSerializerContext : JsonSerializerContext;
