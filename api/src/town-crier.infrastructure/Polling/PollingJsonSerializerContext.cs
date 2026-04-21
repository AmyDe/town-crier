using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Polling;

[JsonSerializable(typeof(PollTriggerPayload))]
internal sealed partial class PollingJsonSerializerContext : JsonSerializerContext;
