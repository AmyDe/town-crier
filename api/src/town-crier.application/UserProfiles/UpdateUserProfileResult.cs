using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.UserProfiles;

public sealed record UpdateUserProfileResult(
    string UserId,
    bool PushEnabled,
    SubscriptionTier Tier);
