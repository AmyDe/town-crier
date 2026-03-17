using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.UserProfiles;

public sealed record ExportUserDataResult(
    string UserId,
    string? Postcode,
    bool PushEnabled,
    SubscriptionTier Tier);
