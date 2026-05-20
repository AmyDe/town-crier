namespace TownCrier.Web.Endpoints;

/// <summary>
/// Response body for <c>POST /v1/subscriptions/verify</c> — the user's
/// entitlement state after the verified Apple transaction has been applied to
/// their Cosmos profile.
/// </summary>
internal sealed record VerifySubscriptionResponse(
    string Tier,
    DateTimeOffset? SubscriptionExpiry,
    IReadOnlyList<string> Entitlements,
    int WatchZoneLimit);
