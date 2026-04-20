using System.Security.Claims;
using TownCrier.Application.UserProfiles;

namespace TownCrier.Web.UserProfiles;

/// <summary>
/// Middleware that updates <c>UserProfile.LastActiveAt</c> on every authenticated
/// request so the daily DormantAccountCleanup worker can delete accounts idle
/// for 12+ months. Enforces UK GDPR Art. 5(1)(e) storage limitation.
///
/// The handler it delegates to is responsible for deduplicating writes within a
/// 24-hour window — calls here are cheap (single Cosmos read per request).
/// Exceptions are swallowed so a transient Cosmos failure never turns a
/// successful API call into a 500.
/// </summary>
internal sealed partial class RecordUserActivityMiddleware(
    RequestDelegate next,
    ILogger<RecordUserActivityMiddleware> logger)
{
    public async Task InvokeAsync(HttpContext context, RecordUserActivityCommandHandler handler, TimeProvider timeProvider)
    {
        await next(context).ConfigureAwait(false);

        var principal = context.User;
        if (principal?.Identity?.IsAuthenticated != true)
        {
            return;
        }

        var userId = principal.FindFirstValue("sub");
        if (string.IsNullOrWhiteSpace(userId))
        {
            return;
        }

        try
        {
            await handler
                .HandleAsync(new RecordUserActivityCommand(userId, timeProvider.GetUtcNow()), context.RequestAborted)
                .ConfigureAwait(false);
        }
#pragma warning disable CA1031 // Activity recording must never break the request
        catch (Exception ex)
#pragma warning restore CA1031
        {
            LogRecordActivityFailure(logger, ex, userId);
        }
    }

    [LoggerMessage(Level = LogLevel.Warning, Message = "Failed to record user activity for {UserId}")]
    private static partial void LogRecordActivityFailure(ILogger logger, Exception exception, string userId);
}
