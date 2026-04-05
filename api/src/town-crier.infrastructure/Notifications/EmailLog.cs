using Microsoft.Extensions.Logging;

namespace TownCrier.Infrastructure.Notifications;

internal static partial class EmailLog
{
    [LoggerMessage(Level = LogLevel.Error, Message = "Failed to send digest email to {Email}")]
    internal static partial void DigestSendFailed(ILogger logger, string email, Exception exception);

    [LoggerMessage(Level = LogLevel.Error, Message = "Failed to send notification email to {Email}")]
    internal static partial void NotificationSendFailed(ILogger logger, string email, Exception exception);
}
