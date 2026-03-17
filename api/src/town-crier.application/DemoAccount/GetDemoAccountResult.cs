using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.DemoAccount;

public sealed record GetDemoAccountResult(
    string UserId,
    SubscriptionTier Tier,
    DemoWatchZoneResult WatchZone,
    IReadOnlyList<DemoApplicationResult> Applications);
