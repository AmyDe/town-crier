using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.PlanIt;

[JsonSerializable(typeof(PlanItResponse))]
[JsonSerializable(typeof(List<PlanItApplicationRecord>))]
[JsonSerializable(typeof(PlanItAreasResponse))]
[JsonSerializable(typeof(List<PlanItAreaRecord>))]
internal sealed partial class PlanItJsonSerializerContext : JsonSerializerContext;
