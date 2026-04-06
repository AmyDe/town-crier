using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.UserProfiles;

public sealed record GetUserProfileResult(
    string UserId,
    bool PushEnabled,
    DayOfWeek DigestDay,
    bool EmailDigestEnabled,
    bool EmailInstantEnabled,
    SubscriptionTier Tier);
