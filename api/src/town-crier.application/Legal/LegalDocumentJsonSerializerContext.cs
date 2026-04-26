using System.Text.Json.Serialization;

namespace TownCrier.Application.Legal;

[JsonSourceGenerationOptions(PropertyNamingPolicy = JsonKnownNamingPolicy.CamelCase)]
[JsonSerializable(typeof(GetLegalDocumentResult))]
internal sealed partial class LegalDocumentJsonSerializerContext : JsonSerializerContext;
