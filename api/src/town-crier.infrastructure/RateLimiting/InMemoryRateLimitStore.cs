using System.Collections.Concurrent;
using TownCrier.Application.RateLimiting;

namespace TownCrier.Infrastructure.RateLimiting;

public sealed class InMemoryRateLimitStore(TimeProvider timeProvider) : IRateLimitStore
{
    private readonly ConcurrentDictionary<string, ConcurrentQueue<long>> requests = new();

    public Task<RateLimitResult> CheckAndIncrementAsync(string clientId, int maxRequests, TimeSpan window, CancellationToken ct)
    {
        var now = timeProvider.GetUtcNow().ToUnixTimeMilliseconds();
        var windowStart = now - (long)window.TotalMilliseconds;

        var queue = this.requests.GetOrAdd(clientId, _ => new ConcurrentQueue<long>());

        // Evict expired entries
        while (queue.TryPeek(out var oldest) && oldest < windowStart)
        {
            queue.TryDequeue(out _);
        }

        if (queue.Count >= maxRequests)
        {
            // Find the earliest entry to calculate when the window opens
            queue.TryPeek(out var earliestTimestamp);
            var retryAfterMs = earliestTimestamp + (long)window.TotalMilliseconds - now;
            var retryAfter = TimeSpan.FromMilliseconds(Math.Max(retryAfterMs, 1));

            return Task.FromResult(new RateLimitResult(false, 0, retryAfter));
        }

        queue.Enqueue(now);
        var remaining = maxRequests - queue.Count;

        return Task.FromResult(new RateLimitResult(true, remaining, TimeSpan.Zero));
    }
}
