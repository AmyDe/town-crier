namespace TownCrier.Domain.OfferCodes;

public sealed class OfferCodeAlreadyRedeemedException : Exception
{
    public OfferCodeAlreadyRedeemedException(string code)
        : base($"Offer code '{code}' has already been redeemed.")
    {
    }

    public OfferCodeAlreadyRedeemedException(string message, Exception innerException)
        : base(message, innerException)
    {
    }

    public OfferCodeAlreadyRedeemedException()
        : base("Offer code has already been redeemed.")
    {
    }
}
