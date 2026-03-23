using System.Text.Json.Serialization;
using TownCrier.Domain.Geocoding;
using TownCrier.Domain.UserProfiles;
using TownCrier.Infrastructure.Groups;

namespace TownCrier.Infrastructure.Cosmos;

[JsonSerializable(typeof(Coordinates))]
[JsonSerializable(typeof(NotificationPreferences))]
[JsonSerializable(typeof(ZoneNotificationPreferences))]
[JsonSerializable(typeof(GroupDocument))]
[JsonSerializable(typeof(List<GroupDocument>))]
[JsonSerializable(typeof(GroupMemberDocument))]
[JsonSerializable(typeof(List<GroupMemberDocument>))]
[JsonSerializable(typeof(GroupInvitationDocument))]
[JsonSerializable(typeof(List<GroupInvitationDocument>))]
internal sealed partial class CosmosJsonSerializerContext : JsonSerializerContext;
