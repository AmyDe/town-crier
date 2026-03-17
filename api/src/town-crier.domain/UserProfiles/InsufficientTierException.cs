namespace TownCrier.Domain.UserProfiles;

public sealed class InsufficientTierException : Exception
{
    public InsufficientTierException(string message)
        : base(message)
    {
    }

    public InsufficientTierException(string message, Exception innerException)
        : base(message, innerException)
    {
    }

    public InsufficientTierException()
        : base("This feature requires a higher subscription tier.")
    {
    }
}
