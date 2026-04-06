namespace TownCrier.Infrastructure.PlanIt;

public sealed record PlanItRetryOptions
{
    public int MaxRetries { get; init; } = 5;

    public double BaseDelaySeconds { get; init; } = 1;

    public TimeSpan BaseDelay => TimeSpan.FromSeconds(this.BaseDelaySeconds);
}
