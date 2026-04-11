using TownCrier.Domain.UserProfiles;

namespace TownCrier.Domain.Subscriptions;

public static class ProductMapping
{
    public static SubscriptionTier ToTier(string productId) => productId switch
    {
        "uk.co.towncrier.personal.monthly" => SubscriptionTier.Personal,
        "uk.co.towncrier.pro.monthly" => SubscriptionTier.Pro,
        _ => throw new ArgumentException($"Unknown App Store product ID: '{productId}'", nameof(productId)),
    };
}
