using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.GovUkPlanningData;

[JsonSerializable(typeof(GovUkEntityResponse))]
[JsonSerializable(typeof(List<GovUkEntity>))]
internal sealed partial class GovUkPlanningDataJsonSerializerContext : JsonSerializerContext;
