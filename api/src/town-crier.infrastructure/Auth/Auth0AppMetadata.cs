using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Auth;

internal sealed record Auth0AppMetadata(
    [property: JsonPropertyName("subscription_tier")] string SubscriptionTier);
