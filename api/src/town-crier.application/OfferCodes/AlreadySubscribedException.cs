namespace TownCrier.Application.OfferCodes;

public sealed class AlreadySubscribedException : Exception
{
    public AlreadySubscribedException()
        : base("User already has an active subscription; offer codes are only available to free-tier users.")
    {
    }

    public AlreadySubscribedException(string message)
        : base(message)
    {
    }

    public AlreadySubscribedException(string message, Exception innerException)
        : base(message, innerException)
    {
    }
}
