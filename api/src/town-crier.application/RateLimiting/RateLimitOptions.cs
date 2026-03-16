namespace TownCrier.Application.RateLimiting;

public sealed class RateLimitOptions
{
    public TimeSpan Window { get; set; } = TimeSpan.FromMinutes(1);

    public int FreeTierLimit { get; set; } = 60;

    public int PaidTierLimit { get; set; } = 600;
}
