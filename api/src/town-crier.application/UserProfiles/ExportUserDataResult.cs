using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.UserProfiles;

public sealed record ExportUserDataResult(
    string UserId,
    bool PushEnabled,
    SubscriptionTier Tier);
