using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Notifications;

[JsonSerializable(typeof(ApnsJwtHeader))]
[JsonSerializable(typeof(ApnsJwtPayload))]
[JsonSerializable(typeof(ApnsAlertPayload))]
[JsonSerializable(typeof(ApnsDigestPayload))]
[JsonSerializable(typeof(ApnsErrorResponse))]
internal sealed partial class ApnsJsonSerializerContext : JsonSerializerContext;
