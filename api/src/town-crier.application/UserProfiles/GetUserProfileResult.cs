using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.UserProfiles;

public sealed record GetUserProfileResult(
    string UserId,
    string? Postcode,
    bool PushEnabled,
    SubscriptionTier Tier);
