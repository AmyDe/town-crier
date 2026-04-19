using Microsoft.Extensions.Logging;

namespace TownCrier.Infrastructure.Notifications;

internal static partial class EmailLog
{
    [LoggerMessage(Level = LogLevel.Error, Message = "Failed to send digest email for user {UserId}")]
    internal static partial void DigestSendFailed(ILogger logger, string userId, Exception exception);

    [LoggerMessage(Level = LogLevel.Error, Message = "Failed to send notification email for user {UserId}")]
    internal static partial void NotificationSendFailed(ILogger logger, string userId, Exception exception);
}
