namespace TownCrier.Application.WatchZones;

public sealed class WatchZoneNotFoundException : Exception
{
    public WatchZoneNotFoundException()
        : base("Watch zone not found.")
    {
    }

    public WatchZoneNotFoundException(string message)
        : base(message)
    {
    }

    public WatchZoneNotFoundException(string message, Exception innerException)
        : base(message, innerException)
    {
    }
}
