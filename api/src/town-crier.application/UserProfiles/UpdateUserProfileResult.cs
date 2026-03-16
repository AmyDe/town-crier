using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.UserProfiles;

public sealed record UpdateUserProfileResult(
    string UserId,
    string? Postcode,
    bool PushEnabled,
    SubscriptionTier Tier);
