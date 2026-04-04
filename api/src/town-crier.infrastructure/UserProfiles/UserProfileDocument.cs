using TownCrier.Domain.UserProfiles;

namespace TownCrier.Infrastructure.UserProfiles;

internal sealed class UserProfileDocument
{
    public required string Id { get; init; }

    public required string UserId { get; init; }

    public string? Email { get; init; }

    public string? Postcode { get; init; }

    public required bool PushEnabled { get; init; }

    public required DayOfWeek DigestDay { get; init; }

    public bool EmailDigestEnabled { get; init; } = true;

    public bool EmailInstantEnabled { get; init; }

    public required Dictionary<string, ZoneNotificationPreferences> ZonePreferences { get; init; }

    public required string Tier { get; init; }

    public DateTimeOffset? SubscriptionExpiry { get; init; }

    public string? OriginalTransactionId { get; init; }

    public DateTimeOffset? GracePeriodExpiry { get; init; }

    public static UserProfileDocument FromDomain(UserProfile profile)
    {
        ArgumentNullException.ThrowIfNull(profile);

        return new UserProfileDocument
        {
            Id = profile.UserId,
            UserId = profile.UserId,
            Email = profile.Email,
            Postcode = profile.Postcode,
            PushEnabled = profile.NotificationPreferences.PushEnabled,
            DigestDay = profile.NotificationPreferences.DigestDay,
            EmailDigestEnabled = profile.NotificationPreferences.EmailDigestEnabled,
            EmailInstantEnabled = profile.NotificationPreferences.EmailInstantEnabled,
            ZonePreferences = new Dictionary<string, ZoneNotificationPreferences>(profile.AllZonePreferences),
            Tier = profile.Tier.ToString(),
            SubscriptionExpiry = profile.SubscriptionExpiry,
            OriginalTransactionId = profile.OriginalTransactionId,
            GracePeriodExpiry = profile.GracePeriodExpiry,
        };
    }

    public UserProfile ToDomain()
    {
        var tier = Enum.Parse<SubscriptionTier>(this.Tier);
        var notificationPreferences = new NotificationPreferences(
            this.PushEnabled,
            this.DigestDay,
            this.EmailDigestEnabled,
            this.EmailInstantEnabled);

        return UserProfile.Reconstitute(
            this.UserId,
            this.Email,
            this.Postcode,
            notificationPreferences,
            this.ZonePreferences,
            tier,
            this.SubscriptionExpiry,
            this.OriginalTransactionId,
            this.GracePeriodExpiry);
    }
}
