using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.Notifications;

internal sealed class UserProfileBuilder
{
    private string userId = "user-1";
    private string? email;
    private SubscriptionTier tier = SubscriptionTier.Free;
    private bool pushEnabled = true;
    private DayOfWeek digestDay = DayOfWeek.Monday;
    private bool emailDigestEnabled = true;
    private bool emailInstantEnabled;

    public UserProfileBuilder WithUserId(string userId)
    {
        this.userId = userId;
        return this;
    }

    public UserProfileBuilder WithEmail(string? email)
    {
        this.email = email;
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

    public UserProfileBuilder WithDigestDay(DayOfWeek day)
    {
        this.digestDay = day;
        return this;
    }

    public UserProfileBuilder WithEmailDigestEnabled(bool enabled)
    {
        this.emailDigestEnabled = enabled;
        return this;
    }

    public UserProfileBuilder WithEmailInstantEnabled(bool enabled)
    {
        this.emailInstantEnabled = enabled;
        return this;
    }

    public UserProfile Build()
    {
        var profile = UserProfile.Register(this.userId, this.email);
        profile.UpdatePreferences(
            new NotificationPreferences(
                this.pushEnabled,
                this.digestDay,
                this.emailDigestEnabled,
                this.emailInstantEnabled));

        if (this.tier != SubscriptionTier.Free)
        {
            profile.ActivateSubscription(this.tier, DateTimeOffset.UtcNow.AddYears(1));
        }

        return profile;
    }
}
