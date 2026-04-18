using TownCrier.Domain.OfferCodes;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Infrastructure.OfferCodes;

internal sealed class OfferCodeDocument
{
    public required string Id { get; init; }

    public required string Code { get; init; }

    public required string Tier { get; init; }

    public required int DurationDays { get; init; }

    public required DateTimeOffset CreatedAt { get; init; }

    public string? RedeemedByUserId { get; init; }

    public DateTimeOffset? RedeemedAt { get; init; }

    public static OfferCodeDocument FromDomain(OfferCode code)
    {
        ArgumentNullException.ThrowIfNull(code);

        return new OfferCodeDocument
        {
            Id = code.Code,
            Code = code.Code,
            Tier = code.Tier.ToString(),
            DurationDays = code.DurationDays,
            CreatedAt = code.CreatedAt,
            RedeemedByUserId = code.RedeemedByUserId,
            RedeemedAt = code.RedeemedAt,
        };
    }

    public OfferCode ToDomain()
    {
        var tier = Enum.Parse<SubscriptionTier>(this.Tier);
        return new OfferCode(
            this.Code,
            tier,
            this.DurationDays,
            this.CreatedAt,
            this.RedeemedByUserId,
            this.RedeemedAt);
    }
}
