using System.Globalization;
using Microsoft.Extensions.Options;
using TownCrier.Application.RateLimiting;

namespace TownCrier.Web.RateLimiting;

internal sealed class RateLimitMiddleware(RequestDelegate next, IRateLimitStore store, IOptions<RateLimitOptions> options)
{
    public async Task InvokeAsync(HttpContext context)
    {
        var userId = context.User.FindFirst("sub")?.Value;

        // Skip rate limiting for anonymous requests
        if (string.IsNullOrEmpty(userId))
        {
            await next(context).ConfigureAwait(false);
            return;
        }

        var config = options.Value;
        var tier = context.User.FindFirst("subscription_tier")?.Value;
        var limit = string.Equals(tier, "paid", StringComparison.OrdinalIgnoreCase)
            ? config.PaidTierLimit
            : config.FreeTierLimit;

        var result = await store.CheckAndIncrementAsync(userId, limit, config.Window, context.RequestAborted).ConfigureAwait(false);

        context.Response.Headers["X-RateLimit-Limit"] = limit.ToString(CultureInfo.InvariantCulture);
        context.Response.Headers["X-RateLimit-Remaining"] = result.Remaining.ToString(CultureInfo.InvariantCulture);

        if (!result.IsAllowed)
        {
            context.Response.StatusCode = StatusCodes.Status429TooManyRequests;
            context.Response.Headers.RetryAfter = ((int)Math.Ceiling(result.RetryAfter.TotalSeconds)).ToString(CultureInfo.InvariantCulture);
            return;
        }

        await next(context).ConfigureAwait(false);
    }
}
