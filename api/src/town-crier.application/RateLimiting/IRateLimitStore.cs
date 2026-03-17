namespace TownCrier.Application.RateLimiting;

public interface IRateLimitStore
{
    Task<RateLimitResult> CheckAndIncrementAsync(string clientId, int maxRequests, TimeSpan window, CancellationToken ct);
}

public sealed record RateLimitResult(bool IsAllowed, int Remaining, TimeSpan RetryAfter);
