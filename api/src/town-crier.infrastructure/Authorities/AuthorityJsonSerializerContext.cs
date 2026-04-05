using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Authorities;

[JsonSerializable(typeof(List<AuthorityRecord>))]
internal sealed partial class AuthorityJsonSerializerContext : JsonSerializerContext;
