using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Notifications;

[JsonSerializable(typeof(ApnsJwtHeader))]
[JsonSerializable(typeof(ApnsJwtPayload))]
internal sealed partial class ApnsJsonSerializerContext : JsonSerializerContext;
