using TownCrier.Domain.UserProfiles;

namespace TownCrier.Web.Endpoints;

internal sealed record GenerateOfferCodesRequest(int Count, SubscriptionTier Tier, int DurationDays);
