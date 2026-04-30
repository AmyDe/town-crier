using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.UserProfiles;

public sealed record UpdateUserProfileResult(
    string UserId,
    bool PushEnabled,
    DayOfWeek DigestDay,
    bool EmailDigestEnabled,
    bool SavedDecisionPush,
    bool SavedDecisionEmail,
    SubscriptionTier Tier);
