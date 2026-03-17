using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.Notifications;

internal sealed class UserProfileBuilder
{
    private string userId = "user-1";
    private SubscriptionTier tier = SubscriptionTier.Free;
    private bool pushEnabled = true;

    public UserProfileBuilder WithUserId(string userId)
    {
        this.userId = userId;
        return this;
    }

    public UserProfileBuilder WithTier(SubscriptionTier tier)
    {
        this.tier = tier;
        return this;
    }

    public UserProfileBuilder WithPushEnabled(bool enabled)
    {
        this.pushEnabled = enabled;
        return this;
    }

    public UserProfile Build()
    {
        var profile = UserProfile.Register(this.userId);
        profile.UpdatePreferences(
            postcode: null,
            new NotificationPreferences(this.pushEnabled));

        if (this.tier != SubscriptionTier.Free)
        {
            profile.ActivateSubscription(this.tier, DateTimeOffset.UtcNow.AddYears(1));
        }

        return profile;
    }
}
