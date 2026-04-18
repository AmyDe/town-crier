using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.OfferCodes;

public sealed record GenerateOfferCodesCommand(int Count, SubscriptionTier Tier, int DurationDays);
