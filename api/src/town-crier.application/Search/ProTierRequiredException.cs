namespace TownCrier.Application.Search;

public sealed class ProTierRequiredException : Exception
{
    public ProTierRequiredException()
        : base("This feature requires a Pro subscription.")
    {
    }

    public ProTierRequiredException(string message)
        : base(message)
    {
    }

    public ProTierRequiredException(string message, Exception innerException)
        : base(message, innerException)
    {
    }
}
