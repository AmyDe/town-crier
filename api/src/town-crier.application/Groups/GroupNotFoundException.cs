namespace TownCrier.Application.Groups;

public sealed class GroupNotFoundException : Exception
{
    public GroupNotFoundException()
        : base("Group not found.")
    {
    }

    public GroupNotFoundException(string message)
        : base(message)
    {
    }

    public GroupNotFoundException(string message, Exception innerException)
        : base(message, innerException)
    {
    }
}
