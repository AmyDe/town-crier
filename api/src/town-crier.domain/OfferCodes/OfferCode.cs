using TownCrier.Domain.UserProfiles;

namespace TownCrier.Domain.OfferCodes;

public sealed class OfferCode
{
    private const string CrockfordBase32 = "0123456789ABCDEFGHJKMNPQRSTVWXYZ";

    public OfferCode(string code, SubscriptionTier tier, int durationDays, DateTimeOffset createdAt)
    {
        ArgumentException.ThrowIfNullOrEmpty(code);

        if (!IsValidCanonicalCode(code))
        {
            throw new ArgumentException(
                $"Code '{code}' is not a valid canonical offer code (12 chars, Crockford base32).",
                nameof(code));
        }

        if (tier == SubscriptionTier.Free)
        {
            throw new ArgumentException("Offer codes cannot grant the Free tier.", nameof(tier));
        }

        if (durationDays < 1 || durationDays > 365)
        {
            throw new ArgumentOutOfRangeException(
                nameof(durationDays),
                durationDays,
                "Duration must be between 1 and 365 days.");
        }

        this.Code = code;
        this.Tier = tier;
        this.DurationDays = durationDays;
        this.CreatedAt = createdAt;
    }

    // Rehydration ctor for repository
    public OfferCode(
        string code,
        SubscriptionTier tier,
        int durationDays,
        DateTimeOffset createdAt,
        string? redeemedByUserId,
        DateTimeOffset? redeemedAt)
        : this(code, tier, durationDays, createdAt)
    {
        this.RedeemedByUserId = redeemedByUserId;
        this.RedeemedAt = redeemedAt;
    }

    public string Code { get; }

    public SubscriptionTier Tier { get; }

    public int DurationDays { get; }

    public DateTimeOffset CreatedAt { get; }

    public string? RedeemedByUserId { get; private set; }

    public DateTimeOffset? RedeemedAt { get; private set; }

    public bool IsRedeemed => this.RedeemedByUserId is not null;

    public void Redeem(string userId, DateTimeOffset now)
    {
        ArgumentException.ThrowIfNullOrEmpty(userId);

        if (this.IsRedeemed)
        {
            throw new OfferCodeAlreadyRedeemedException(this.Code);
        }

        this.RedeemedByUserId = userId;
        this.RedeemedAt = now;
    }

    private static bool IsValidCanonicalCode(string code)
    {
        if (code.Length != 12)
        {
            return false;
        }

        foreach (var c in code)
        {
            if (CrockfordBase32.IndexOf(c, StringComparison.Ordinal) < 0)
            {
                return false;
            }
        }

        return true;
    }
}
