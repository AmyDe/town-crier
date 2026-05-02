using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.PlanIt;

[JsonSerializable(typeof(PlanItResponse))]
[JsonSerializable(typeof(List<PlanItApplicationRecord>))]
[JsonSerializable(typeof(PlanItApplicationRecord))]
internal sealed partial class PlanItJsonSerializerContext : JsonSerializerContext;
