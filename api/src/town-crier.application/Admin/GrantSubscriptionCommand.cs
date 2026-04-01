using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Admin;

public sealed record GrantSubscriptionCommand(string Email, SubscriptionTier Tier);
