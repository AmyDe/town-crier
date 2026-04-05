namespace TownCrier.Domain.UserProfiles;

public sealed class UserProfile
{
    private readonly Dictionary<string, ZoneNotificationPreferences> zonePreferences = [];

    private UserProfile(
        string userId,
        string? email,
        NotificationPreferences notificationPreferences,
        SubscriptionTier tier,
        DateTimeOffset? subscriptionExpiry,
        string? originalTransactionId,
        DateTimeOffset? gracePeriodExpiry)
    {
        this.UserId = userId;
        this.Email = email;
        this.NotificationPreferences = notificationPreferences;
        this.Tier = tier;
        this.SubscriptionExpiry = subscriptionExpiry;
        this.OriginalTransactionId = originalTransactionId;
        this.GracePeriodExpiry = gracePeriodExpiry;
    }

    public string UserId { get; }

    public string? Email { get; }

    public NotificationPreferences NotificationPreferences { get; private set; }

    public IReadOnlyDictionary<string, ZoneNotificationPreferences> AllZonePreferences => this.zonePreferences;

    public SubscriptionTier Tier { get; private set; }

    public DateTimeOffset? SubscriptionExpiry { get; private set; }

    public string? OriginalTransactionId { get; private set; }

    public DateTimeOffset? GracePeriodExpiry { get; private set; }

    public static UserProfile Register(string userId, string? email = null)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);

        return new UserProfile(
            userId,
            email,
            notificationPreferences: NotificationPreferences.Default,
            tier: SubscriptionTier.Free,
            subscriptionExpiry: null,
            originalTransactionId: null,
            gracePeriodExpiry: null);
    }

    public void UpdatePreferences(NotificationPreferences notificationPreferences)
    {
        this.NotificationPreferences = notificationPreferences;
    }

    public void LinkOriginalTransactionId(string originalTransactionId)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(originalTransactionId);
        this.OriginalTransactionId = originalTransactionId;
    }

    public void ActivateSubscription(SubscriptionTier tier, DateTimeOffset expiresDate)
    {
        if (tier == SubscriptionTier.Free)
        {
            throw new ArgumentException("Cannot activate a free subscription.", nameof(tier));
        }

        this.Tier = tier;
        this.SubscriptionExpiry = expiresDate;
        this.GracePeriodExpiry = null;
    }

    public void RenewSubscription(DateTimeOffset newExpiresDate)
    {
        this.SubscriptionExpiry = newExpiresDate;
        this.GracePeriodExpiry = null;
    }

    public void ExpireSubscription()
    {
        this.Tier = SubscriptionTier.Free;
        this.SubscriptionExpiry = null;
        this.GracePeriodExpiry = null;
    }

    public void EnterGracePeriod(DateTimeOffset gracePeriodExpiry)
    {
        this.GracePeriodExpiry = gracePeriodExpiry;
    }

    public ZoneNotificationPreferences GetZonePreferences(string zoneId)
    {
        return this.zonePreferences.TryGetValue(zoneId, out var prefs)
            ? prefs
            : ZoneNotificationPreferences.Default;
    }

    public void SetZonePreferences(string zoneId, ZoneNotificationPreferences preferences)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(zoneId);
        ArgumentNullException.ThrowIfNull(preferences);

        if (this.Tier == SubscriptionTier.Free && (preferences.StatusChanges || preferences.DecisionUpdates))
        {
            throw new InsufficientTierException(
                "Status changes and decision updates require a Pro subscription.");
        }

        this.zonePreferences[zoneId] = preferences;
    }

    internal static UserProfile Reconstitute(
        string userId,
        string? email,
        NotificationPreferences notificationPreferences,
        Dictionary<string, ZoneNotificationPreferences> zonePreferences,
        SubscriptionTier tier,
        DateTimeOffset? subscriptionExpiry,
        string? originalTransactionId,
        DateTimeOffset? gracePeriodExpiry)
    {
        var profile = new UserProfile(
            userId,
            email,
            notificationPreferences,
            tier,
            subscriptionExpiry,
            originalTransactionId,
            gracePeriodExpiry);

        foreach (var (zoneId, prefs) in zonePreferences)
        {
            profile.zonePreferences[zoneId] = prefs;
        }

        return profile;
    }
}
