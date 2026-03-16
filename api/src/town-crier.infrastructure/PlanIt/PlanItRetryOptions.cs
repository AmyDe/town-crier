namespace TownCrier.Infrastructure.PlanIt;

public sealed record PlanItRetryOptions
{
    public int MaxRetries { get; init; } = 5;

    public TimeSpan BaseDelay { get; init; } = TimeSpan.FromSeconds(1);
}
