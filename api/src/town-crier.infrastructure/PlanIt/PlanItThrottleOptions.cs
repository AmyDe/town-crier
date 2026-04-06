namespace TownCrier.Infrastructure.PlanIt;

public sealed record PlanItThrottleOptions
{
    public TimeSpan DelayBetweenRequests { get; init; } = TimeSpan.FromSeconds(2);
}
