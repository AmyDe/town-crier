namespace TownCrier.Application.OfferCodes;

public sealed class OfferCodeNotFoundException : Exception
{
    public OfferCodeNotFoundException()
    {
    }

    public OfferCodeNotFoundException(string code)
        : base($"Offer code '{code}' was not found.")
    {
    }

    public OfferCodeNotFoundException(string message, Exception innerException)
        : base(message, innerException)
    {
    }
}
