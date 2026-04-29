using TownCrier.Domain.UserProfiles;

namespace TownCrier.Infrastructure.UserProfiles;

internal sealed class UserProfileDocument
{
    public required string Id { get; init; }

    public required string UserId { get; init; }

    public string? Email { get; init; }

    public required bool PushEnabled { get; init; }

    public required DayOfWeek DigestDay { get; init; }

    // Nullable so legacy Cosmos documents predating emailDigestEnabled (tc-ho5w) hydrate
    // as opt-in to email digests — preserving prior default-on behaviour. The
    // System.Text.Json source generator sets `bool` properties to `default(bool)` (false)
    // when the JSON field is missing — even when a property initialiser declares `= true`
    // — so we use `bool?` and coalesce at `ToDomain` time.
    public bool? EmailDigestEnabled { get; init; }

    // Legacy field — kept for backward compatibility with existing Cosmos documents.
    // No longer written; ignored during domain reconstitution.
    public bool EmailInstantEnabled { get; init; }

    public required Dictionary<string, ZoneNotificationPreferences> ZonePreferences { get; init; }

    public required string Tier { get; init; }

    public DateTimeOffset? SubscriptionExpiry { get; init; }

    public string? OriginalTransactionId { get; init; }

    public DateTimeOffset? GracePeriodExpiry { get; init; }

    // Defaults to MinValue so historical documents (pre-retention work) are treated
    // as never-active and cleanly deleted on first dormant-cleanup pass. Existing
    // documents lacking the field get the default at deserialisation time; the
    // first authenticated request refreshes it via RecordActivity.
    public DateTimeOffset LastActiveAt { get; init; }

    public static UserProfileDocument FromDomain(UserProfile profile)
    {
        ArgumentNullException.ThrowIfNull(profile);

        return new UserProfileDocument
        {
            Id = profile.UserId,
            UserId = profile.UserId,
            Email = profile.Email,
            PushEnabled = profile.NotificationPreferences.PushEnabled,
            DigestDay = profile.NotificationPreferences.DigestDay,
            EmailDigestEnabled = profile.NotificationPreferences.EmailDigestEnabled,
            ZonePreferences = new Dictionary<string, ZoneNotificationPreferences>(profile.AllZonePreferences),
            Tier = profile.Tier.ToString(),
            SubscriptionExpiry = profile.SubscriptionExpiry,
            OriginalTransactionId = profile.OriginalTransactionId,
            GracePeriodExpiry = profile.GracePeriodExpiry,
            LastActiveAt = profile.LastActiveAt,
        };
    }

    public UserProfile ToDomain()
    {
        var tier = Enum.Parse<SubscriptionTier>(this.Tier);
        var notificationPreferences = new NotificationPreferences(
            this.PushEnabled,
            this.DigestDay,
            this.EmailDigestEnabled ?? true);

        return UserProfile.Reconstitute(
            this.UserId,
            this.Email,
            notificationPreferences,
            this.ZonePreferences,
            tier,
            this.SubscriptionExpiry,
            this.OriginalTransactionId,
            this.GracePeriodExpiry,
            this.LastActiveAt);
    }
}
