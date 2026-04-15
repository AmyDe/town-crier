using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Admin;

public sealed record ListUsersItem(string UserId, string? Email, SubscriptionTier Tier);
