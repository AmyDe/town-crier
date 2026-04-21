using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.ServiceBus;

[JsonSerializable(typeof(BrokerProperties))]
internal sealed partial class ServiceBusJsonSerializerContext : JsonSerializerContext;
