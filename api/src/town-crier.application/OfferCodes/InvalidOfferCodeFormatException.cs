namespace TownCrier.Application.OfferCodes;

public sealed class InvalidOfferCodeFormatException : Exception
{
    public InvalidOfferCodeFormatException()
    {
    }

    public InvalidOfferCodeFormatException(string reason)
        : base(reason)
    {
    }

    public InvalidOfferCodeFormatException(string message, Exception innerException)
        : base(message, innerException)
    {
    }
}
