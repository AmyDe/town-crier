namespace TownCrier.Infrastructure.PlanIt;

public sealed record PlanItThrottleOptions
{
    public double DelayBetweenRequestsSeconds { get; init; } = 2;

    public TimeSpan DelayBetweenRequests => TimeSpan.FromSeconds(this.DelayBetweenRequestsSeconds);
}
