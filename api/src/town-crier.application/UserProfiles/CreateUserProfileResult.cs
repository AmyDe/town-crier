using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.UserProfiles;

public sealed record CreateUserProfileResult(
    string UserId,
    string? Postcode,
    bool PushEnabled,
    SubscriptionTier Tier);
