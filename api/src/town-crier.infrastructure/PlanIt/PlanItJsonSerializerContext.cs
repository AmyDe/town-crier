using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.PlanIt;

[JsonSerializable(typeof(PlanItResponse))]
[JsonSerializable(typeof(List<PlanItApplicationRecord>))]
internal sealed partial class PlanItJsonSerializerContext : JsonSerializerContext;
