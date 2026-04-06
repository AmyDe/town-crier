using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Auth;

internal sealed record Auth0UpdateMetadataRequest(
    [property: JsonPropertyName("app_metadata")] Auth0AppMetadata AppMetadata);
