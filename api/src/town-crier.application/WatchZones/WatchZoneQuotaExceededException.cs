namespace TownCrier.Application.WatchZones;

public sealed class WatchZoneQuotaExceededException : Exception
{
    public WatchZoneQuotaExceededException(int limit)
        : base($"Watch zone quota exceeded. Maximum allowed: {limit}.")
    {
        this.Limit = limit;
    }

    public WatchZoneQuotaExceededException()
        : base("Watch zone quota exceeded.")
    {
    }

    public WatchZoneQuotaExceededException(string message)
        : base(message)
    {
    }

    public WatchZoneQuotaExceededException(string message, Exception innerException)
        : base(message, innerException)
    {
    }

    public int Limit { get; }
}
