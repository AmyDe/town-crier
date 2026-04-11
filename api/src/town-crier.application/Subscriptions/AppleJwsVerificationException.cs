namespace TownCrier.Application.Subscriptions;

public sealed class AppleJwsVerificationException : Exception
{
    public AppleJwsVerificationException()
    {
    }

    public AppleJwsVerificationException(string message)
        : base(message)
    {
    }

    public AppleJwsVerificationException(string message, Exception innerException)
        : base(message, innerException)
    {
    }
}
