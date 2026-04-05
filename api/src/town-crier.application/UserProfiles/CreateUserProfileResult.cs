using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.UserProfiles;

public sealed record CreateUserProfileResult(
    string UserId,
    bool PushEnabled,
    SubscriptionTier Tier);
