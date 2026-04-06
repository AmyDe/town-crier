namespace TownCrier.Application.Polling;

public sealed record PlanItPollingOptions
{
    public double RateLimitCooldownSeconds { get; init; } = 30;

    public TimeSpan RateLimitCooldown => TimeSpan.FromSeconds(this.RateLimitCooldownSeconds);
}
