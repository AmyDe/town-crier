using System.Globalization;
using System.Security.Claims;
using Microsoft.Extensions.Options;
using TownCrier.Application.RateLimiting;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Web.RateLimiting;

internal sealed class RateLimitMiddleware(
    RequestDelegate next,
    IRateLimitStore store,
    IUserProfileRepository userProfileRepository,
    IOptions<RateLimitOptions> options)
{
    public async Task InvokeAsync(HttpContext context)
    {
        var userId = context.User.FindFirst("sub")?.Value
            ?? context.User.FindFirst(ClaimTypes.NameIdentifier)?.Value;

        // Skip rate limiting for anonymous requests
        if (string.IsNullOrEmpty(userId))
        {
            await next(context).ConfigureAwait(false);
            return;
        }

        var config = options.Value;

        // ADR 0010: Cosmos DB is the single source of truth for entitlements.
        // The subscription_tier JWT claim is at most a non-authoritative cache, so
        // the paid rate limit is driven by the tier stored on the Cosmos UserProfile.
        var profile = await userProfileRepository
            .GetByUserIdAsync(userId, context.RequestAborted)
            .ConfigureAwait(false);
        var isPaid = profile is not null && profile.Tier != SubscriptionTier.Free;
        var limit = isPaid ? config.PaidTierLimit : config.FreeTierLimit;

        var result = await store.CheckAndIncrementAsync(userId, limit, config.Window, context.RequestAborted).ConfigureAwait(false);

        if (!result.IsAllowed)
        {
            context.Response.StatusCode = StatusCodes.Status429TooManyRequests;
            context.Response.Headers.Append("X-RateLimit-Limit", limit.ToString(CultureInfo.InvariantCulture));
            context.Response.Headers.Append("X-RateLimit-Remaining", "0");
            context.Response.Headers.RetryAfter = ((int)Math.Ceiling(result.RetryAfter.TotalSeconds)).ToString(CultureInfo.InvariantCulture);
            return;
        }

        context.Response.OnStarting(() =>
        {
            context.Response.Headers.Append("X-RateLimit-Limit", limit.ToString(CultureInfo.InvariantCulture));
            context.Response.Headers.Append("X-RateLimit-Remaining", result.Remaining.ToString(CultureInfo.InvariantCulture));
            return Task.CompletedTask;
        });

        await next(context).ConfigureAwait(false);
    }
}
