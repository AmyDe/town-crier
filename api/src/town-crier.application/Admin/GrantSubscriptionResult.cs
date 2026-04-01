using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Admin;

public sealed record GrantSubscriptionResult(
    string UserId,
    string? Email,
    SubscriptionTier Tier);
