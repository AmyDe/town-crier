namespace TownCrier.Domain.Groups;

public sealed class UnauthorizedGroupOperationException : Exception
{
    public UnauthorizedGroupOperationException()
        : base("You are not authorized to perform this operation on this group.")
    {
    }

    public UnauthorizedGroupOperationException(string message)
        : base(message)
    {
    }

    public UnauthorizedGroupOperationException(string message, Exception innerException)
        : base(message, innerException)
    {
    }
}
