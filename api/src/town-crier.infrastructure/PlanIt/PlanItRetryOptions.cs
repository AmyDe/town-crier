namespace TownCrier.Infrastructure.PlanIt;

public sealed record PlanItRetryOptions
{
    public int MaxRetries { get; init; } = 3;

    public double InitialBackoffSeconds { get; init; } = 1;

    public double RateLimitBackoffSeconds { get; init; } = 5;

    public TimeSpan InitialBackoff => TimeSpan.FromSeconds(this.InitialBackoffSeconds);

    public TimeSpan RateLimitBackoff => TimeSpan.FromSeconds(this.RateLimitBackoffSeconds);
}
