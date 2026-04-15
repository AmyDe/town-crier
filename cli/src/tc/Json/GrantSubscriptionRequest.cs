namespace Tc.Json;

internal sealed class GrantSubscriptionRequest
{
    public required string Email { get; init; }

    public required string SubscriptionTier { get; init; }
}
