using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.ServiceBus;

[JsonSerializable(typeof(BrokerProperties))]
[JsonSerializable(typeof(QueueCountDetailsResponse))]
internal sealed partial class ServiceBusJsonSerializerContext : JsonSerializerContext;
